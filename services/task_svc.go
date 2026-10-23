package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"

	"k8s.io/apimachinery/pkg/util/wait"
)

type TaskService interface {
	ListTasks([]string, models.TaskQueryParams) ([]*models.Task, int, error)
	GetTask(string) (*models.Task, error)
	CreateTask(models.Task) (*models.Task, error)
	UpdateTask(models.Task) (*models.Task, error)
	DeleteTask(string) error
	GetSchema(string) (map[string]interface{}, error)
	DeleteSchema(string) error
	UpdateSchema(string, map[string]interface{}) (map[string]interface{}, error)
	RunTask(string, string, string) (models.Job, error)
}

type TaskServiceImpl struct {
	db             *gorm.DB
	WebhookService WebhookService
	JobService     JobService
	RunnerService  RunnerService
	config         config.Config
}

func NewTaskService(database *gorm.DB, ws WebhookService, config config.Config, js JobService, rs RunnerService) TaskService {
	return &TaskServiceImpl{
		db:             database,
		WebhookService: ws,
		JobService:     js,
		RunnerService:  rs,
		config:         config,
	}
}

// getTaskByName fetches a task model from the database, translating the
// not found error into the service level error.
func (t *TaskServiceImpl) getTaskByName(name string) (*models.Task, error) {
	var task models.Task

	res := t.db.Where("name = ?", name).First(&task)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", name, res.Error)
	}

	return &task, nil
}

func (t *TaskServiceImpl) ListTasks(authList []string, params models.TaskQueryParams) ([]*models.Task, int, error) {
	var tasks []*models.Task

	if len(authList) == 0 {
		return tasks, 0, nil
	}

	res := t.db.Find(&tasks)
	if res.Error != nil {
		return nil, 0, fmt.Errorf("failed to fetch list of tasks: %w", res.Error)
	}

	var authorised []*models.Task
	if slices.Contains(authList, "*") {
		authorised = tasks
	} else {
		for _, task := range tasks {
			if slices.Contains(authList, task.Name) {
				authorised = append(authorised, task)
			}
		}
	}

	var filtered []*models.Task
	for _, task := range authorised {
		if params.Name != "" && !strings.Contains(strings.ToLower(task.Name), strings.ToLower(params.Name)) {
			continue
		}
		filtered = append(filtered, task)
	}

	total := len(filtered)

	if params.Limit > 0 {
		start := params.Offset
		if start > total {
			start = total
		}
		end := start + params.Limit
		if end > total {
			end = total
		}
		filtered = filtered[start:end]
	}

	return filtered, total, nil
}

func (t *TaskServiceImpl) GetTask(name string) (*models.Task, error) {
	return t.getTaskByName(name)
}

func (t *TaskServiceImpl) CreateTask(task models.Task) (*models.Task, error) {
	err := helpers.ValidateK8sConfigMapName(task.Name)
	if err != nil {
		return nil, fmt.Errorf("task '%s': %w",
			task.Name,
			ErrSvcObjNameK8sConfMap)
	}
	_, err = t.RunnerService.GetRunner(task.Runner)
	if err != nil {
		if errors.Is(err, ErrSvcObjNotFound) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, err)
	}

	if task.Schema != nil {
		jsonData, err := json.Marshal(task.Schema)
		if err != nil {
			return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			errString := strings.ReplaceAll(err.Error(), "\"", "'")
			return nil, fmt.Errorf("task '%s' schema: %w, %s", task.Name, ErrSvcTaskSchemaValidation, errString)
		}
	}

	newTask := models.Task{
		Schema:      task.Schema,
		Name:        task.Name,
		Runner:      task.Runner,
		Command:     task.Command,
		Synchronous: task.Synchronous,
	}

	res := t.db.Create(&newTask)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return nil, fmt.Errorf("task '%s': %w", task.Name, ErrSvcObjExists)
		}
		return nil, fmt.Errorf("task '%s': %w", task.Name, res.Error)
	}

	return &newTask, nil
}

func (t *TaskServiceImpl) UpdateTask(task models.Task) (*models.Task, error) {
	existing, err := t.getTaskByName(task.Name)
	if err != nil {
		return nil, err
	}

	_, err = t.RunnerService.GetRunner(task.Runner)
	if err != nil {
		if errors.Is(err, ErrSvcObjNotFound) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, err)
	}

	if task.Schema != nil {
		jsonData, err := json.Marshal(task.Schema)
		if err != nil {
			return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			errString := strings.ReplaceAll(err.Error(), "\"", "'")
			return nil, fmt.Errorf("task '%s' schema: %w, %s", task.Name, ErrSvcTaskSchemaValidation, errString)
		}
	}

	existing.Schema = task.Schema
	existing.Runner = task.Runner
	existing.Command = task.Command
	existing.Synchronous = task.Synchronous

	if res := t.db.Save(existing); res.Error != nil {
		return nil, fmt.Errorf("task '%s': %w", task.Name, res.Error)
	}

	return existing, nil
}

func (t *TaskServiceImpl) DeleteTask(name string) error {
	res, err := t.WebhookService.ListTaskWebhooks(name)
	if err == nil && len(res) != 0 {
		return fmt.Errorf("cannot delete task %s, please remove associated webhooks first: %w", name, ErrSvcObjInUse)
	}

	task, err := t.getTaskByName(name)
	if err != nil {
		return err
	}

	if err := t.db.Delete(task).Error; err != nil {
		return fmt.Errorf("task '%s': %w", name, err)
	}

	return nil
}

func (t *TaskServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
	task, err := t.getTaskByName(name)
	if err != nil {
		return nil, err
	}

	return task.Schema, nil
}

func (t *TaskServiceImpl) UpdateSchema(name string, schema map[string]interface{}) (map[string]interface{}, error) {
	task, err := t.getTaskByName(name)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("task '%s' schema: %w", name, err)
	}

	err = ValidateSchema(data)
	if err != nil {
		errString := strings.ReplaceAll(err.Error(), "\"", "'")
		return nil, fmt.Errorf("task '%s' schema: %w, %s", name, ErrSvcTaskSchemaValidation, errString)
	}

	task.Schema = schema
	if err := t.db.Save(task).Error; err != nil {
		return nil, fmt.Errorf("task '%s' schema: %w", name, err)
	}

	return schema, nil
}

func (t *TaskServiceImpl) DeleteSchema(name string) error {
	task, err := t.getTaskByName(name)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if task.Schema == nil {
		return nil
	}

	task.Schema = nil
	if err := t.db.Save(task).Error; err != nil {
		return fmt.Errorf("task '%s' schema: %w", name, err)
	}

	return nil
}

func ValidateSchema(schema []byte) error {
	input, err := os.ReadFile("spec.json")
	if err != nil {
		return fmt.Errorf("failed to read schema spec.json file: %w", err)
	}

	output := bytes.ReplaceAll(input, []byte("\"%schema%\""), schema)
	doc, err := loads.Analyzed(output, "2.0")
	if err != nil {
		return fmt.Errorf("failed to load schema spec: %w", err)
	}

	validate.SetContinueOnErrors(true)       // Set global options
	err = validate.Spec(doc, strfmt.Default) // Validates spec with default Swagger 2.0 format definitions

	if err != nil {
		return fmt.Errorf("%v", err)
	}

	return nil
}

func (t *TaskServiceImpl) RunTask(username string, taskName string, extraVars string) (models.Job, error) {
	ctx := context.Background()
	var jobStatus models.Job

	task, err := t.getTaskByName(taskName)
	if err != nil {
		return jobStatus, err
	}
	runnerName := task.Runner

	if task.Schema != nil {
		schemaData, err := json.Marshal(task.Schema)
		if err != nil {
			return jobStatus, fmt.Errorf("task '%s' schema: %w", taskName, err)
		}

		schema := new(spec.Schema)
		_ = json.Unmarshal(schemaData, schema)

		input := map[string]interface{}{}

		// JSON data to validate
		_ = json.Unmarshal([]byte(extraVars), &input)

		// strfmt.Default is the registry of recognized formats
		err = validate.AgainstSchema(schema, input, strfmt.Default)
		if err != nil {
			return models.Job{}, fmt.Errorf("task '%s' schema: %w, %s", taskName, ErrSvcTaskSchemaValidation, err)
		}
	}

	runner, err := t.RunnerService.GetRunnerForExecution(runnerName)
	if err != nil {
		return jobStatus, err
	}
	runnerImage := runner.Image
	gitURL := runner.GitURL
	gitBranch := runner.Branch

	if gitBranch == "" {
		gitBranch = "main"
	}

	// Custom runner secrets are materialised into a short lived Kubernetes
	// secret referenced by the job pod; it is garbage collected with the job.
	envSecret := ""
	if len(runner.Secret) != 0 {
		envSecret, err = helpers.GenerateName("kriten-run")
		if err != nil {
			return jobStatus, err
		}
		if _, err = helpers.CreateOrUpdateSecret(t.config.Kube, envSecret, runner.Secret, "create"); err != nil {
			return jobStatus, fmt.Errorf("task '%s' secrets: %w", taskName, err)
		}
	}

	// The git token is kept in a separate secret only referenced by the init
	// container so that it is never exposed to the task container.
	tokenSecret := ""
	if runner.Token != "" && gitURL != "" {
		tokenSecret, err = helpers.GenerateName("kriten-token")
		if err != nil {
			if envSecret != "" {
				_ = helpers.DeleteSecret(t.config.Kube, envSecret)
			}
			return jobStatus, err
		}
		token := map[string]string{"GIT_TOKEN": runner.Token}
		if _, err = helpers.CreateOrUpdateSecret(t.config.Kube, tokenSecret, token, "create"); err != nil {
			if envSecret != "" {
				_ = helpers.DeleteSecret(t.config.Kube, envSecret)
			}
			return jobStatus, fmt.Errorf("task '%s' git repo token: %w", taskName, err)
		}
	}

	job, err := helpers.CreateJob(
		t.config.Kube,
		taskName,
		envSecret,
		tokenSecret,
		runnerImage,
		username,
		extraVars,
		task.Command,
		gitURL,
		gitBranch,
	)

	if err != nil {
		if envSecret != "" {
			_ = helpers.DeleteSecret(t.config.Kube, envSecret)
		}
		if tokenSecret != "" {
			_ = helpers.DeleteSecret(t.config.Kube, tokenSecret)
		}
		return jobStatus, err
	}

	jobName := job.Name
	jobStatus.Name = jobName

	// Tie the secrets lifetime to the job so they are garbage collected.
	if envSecret != "" {
		if err := helpers.SetSecretOwnerReference(t.config.Kube, envSecret, job); err != nil {
			fmt.Println(err)
		}
	}
	if tokenSecret != "" {
		if err := helpers.SetSecretOwnerReference(t.config.Kube, tokenSecret, job); err != nil {
			fmt.Println(err)
		}
	}

	if task.Synchronous {
		err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 20*time.Second, false,
			func(conditionCtx context.Context) (done bool, err error) {

				job, err := helpers.GetJob(t.config.Kube, jobName)

				if err != nil {
					fmt.Println(err)
					return false, err
				}

				if job.Status.Succeeded != 0 || job.Status.Failed != 0 {
					return true, nil
				}

				return false, nil
			})

		ret, err := t.JobService.GetJob(username, jobName)
		return ret, err
	}

	return jobStatus, nil
}
