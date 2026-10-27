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
	uuid "github.com/satori/go.uuid"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"

	"k8s.io/apimachinery/pkg/util/wait"
)

type TaskExecutor interface {
	ExecuteTask(string, uuid.UUID, string) (models.Job, error)
}

type TaskService interface {
	TaskExecutor
	ListTasks([]string, models.TaskQueryParams) ([]*models.Task, int, error)
	GetTask(uuid.UUID) (*models.Task, error)
	CreateTask(models.AuditMetadata, models.Task) (*models.Task, error)
	UpdateTask(models.AuditMetadata, uuid.UUID, models.Task) (*models.Task, error)
	DeleteTask(models.AuditMetadata, uuid.UUID) error
	GetSchema(uuid.UUID) (map[string]interface{}, error)
	DeleteSchema(models.AuditMetadata, uuid.UUID) error
	UpdateSchema(models.AuditMetadata, uuid.UUID, map[string]interface{}) (map[string]interface{}, error)
	RunTask(models.AuditMetadata, string, uuid.UUID, string) (models.Job, error)
}

type TaskServiceImpl struct {
	db            *gorm.DB
	JobService    JobService
	RunnerService RunnerService
	config        config.Config
	auditService  AuditService
}

func NewTaskService(database *gorm.DB, config config.Config, js JobService, rs RunnerService, als AuditService) TaskService {
	return &TaskServiceImpl{
		db:            database,
		JobService:    js,
		RunnerService: rs,
		config:        config,
		auditService:  als,
	}
}

// getTaskByID fetches a task model from the database, translating the
// not found error into the service level error.
func (t *TaskServiceImpl) getTaskByID(id uuid.UUID) (*models.Task, error) {
	var task models.Task

	res := t.db.Where("id = ?", id).First(&task)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task '%s': %w", id, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", id, res.Error)
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
			if slices.Contains(authList, task.ID.String()) {
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

func (t *TaskServiceImpl) GetTask(id uuid.UUID) (*models.Task, error) {
	return t.getTaskByID(id)
}

func (t *TaskServiceImpl) CreateTask(meta models.AuditMetadata, task models.Task) (*models.Task, error) {
	audit := t.auditService.InitialiseAuditLog(meta, "create", "tasks", task.Name)
	defer func() { t.auditService.CreateAudit(audit) }()

	err := helpers.ValidateK8sObjName(task.Name)
	if err != nil {
		return nil, fmt.Errorf("task '%s': %w",
			task.Name,
			ErrSvcObjNameK8sConfMap)
	}
	_, err = t.RunnerService.GetRunner(task.RunnerID)
	if err != nil {
		if errors.Is(err, ErrSvcObjNotFound) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.RunnerID, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.RunnerID, err)
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
		RunnerID:    task.RunnerID,
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

	audit.Status = "success"
	return &newTask, nil
}

func (t *TaskServiceImpl) UpdateTask(meta models.AuditMetadata, id uuid.UUID, task models.Task) (*models.Task, error) {
	audit := t.auditService.InitialiseAuditLog(meta, "update", "tasks", id.String())
	defer func() { t.auditService.CreateAudit(audit) }()

	existing, err := t.getTaskByID(id)
	if err != nil {
		return nil, err
	}

	_, err = t.RunnerService.GetRunner(task.RunnerID)
	if err != nil {
		if errors.Is(err, ErrSvcObjNotFound) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", existing.Name, task.RunnerID, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", existing.Name, task.RunnerID, err)
	}

	if task.Schema != nil {
		jsonData, err := json.Marshal(task.Schema)
		if err != nil {
			return nil, fmt.Errorf("task '%s' schema: %w", existing.Name, err)
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			errString := strings.ReplaceAll(err.Error(), "\"", "'")
			return nil, fmt.Errorf("task '%s' schema: %w, %s", existing.Name, ErrSvcTaskSchemaValidation, errString)
		}
	}

	existing.Schema = task.Schema
	existing.RunnerID = task.RunnerID
	existing.Command = task.Command
	existing.Synchronous = task.Synchronous

	if res := t.db.Save(existing); res.Error != nil {
		return nil, fmt.Errorf("task '%s': %w", existing.Name, res.Error)
	}

	audit.EventTarget = existing.Name
	audit.Status = "success"
	return existing, nil
}

func (t *TaskServiceImpl) DeleteTask(meta models.AuditMetadata, id uuid.UUID) error {
	audit := t.auditService.InitialiseAuditLog(meta, "delete", "tasks", id.String())
	defer func() { t.auditService.CreateAudit(audit) }()

	task, err := t.getTaskByID(id)
	if err != nil {
		return err
	}

	var webhookCount int64
	if err := t.db.Model(&models.Webhook{}).Where("task_id = ?", task.ID).Count(&webhookCount).Error; err != nil {
		return fmt.Errorf("task '%s' webhooks: %w", task.Name, err)
	}
	if webhookCount != 0 {
		return fmt.Errorf("cannot delete task %s, please remove associated webhooks first: %w", task.Name, ErrSvcObjInUse)
	}

	var schedulerCount int64
	if err := t.db.Model(&models.Scheduler{}).Where("task_id = ?", task.ID).Count(&schedulerCount).Error; err != nil {
		return fmt.Errorf("task '%s' schedulers: %w", task.Name, err)
	}
	if schedulerCount != 0 {
		return fmt.Errorf("cannot delete task %s, please remove associated schedulers first: %w", task.Name, ErrSvcObjInUse)
	}

	if err := t.db.Delete(task).Error; err != nil {
		return fmt.Errorf("task '%s': %w", task.Name, err)
	}

	audit.Status = "success"
	return nil
}

func (t *TaskServiceImpl) GetSchema(id uuid.UUID) (map[string]interface{}, error) {
	task, err := t.getTaskByID(id)
	if err != nil {
		return nil, err
	}

	return task.Schema, nil
}

func (t *TaskServiceImpl) UpdateSchema(meta models.AuditMetadata, id uuid.UUID, schema map[string]interface{}) (map[string]interface{}, error) {
	audit := t.auditService.InitialiseAuditLog(meta, "update_schema", "tasks", id.String())
	defer func() { t.auditService.CreateAudit(audit) }()

	task, err := t.getTaskByID(id)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
	}

	err = ValidateSchema(data)
	if err != nil {
		errString := strings.ReplaceAll(err.Error(), "\"", "'")
		return nil, fmt.Errorf("task '%s' schema: %w, %s", task.Name, ErrSvcTaskSchemaValidation, errString)
	}

	task.Schema = schema
	if err := t.db.Save(task).Error; err != nil {
		return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
	}

	audit.Status = "success"
	return schema, nil
}

func (t *TaskServiceImpl) DeleteSchema(meta models.AuditMetadata, id uuid.UUID) error {
	audit := t.auditService.InitialiseAuditLog(meta, "delete_schema", "tasks", id.String())
	defer func() { t.auditService.CreateAudit(audit) }()

	task, err := t.getTaskByID(id)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if task.Schema != nil {
		task.Schema = nil
		if err := t.db.Save(task).Error; err != nil {
			return fmt.Errorf("task '%s' schema: %w", task.Name, err)
		}
	}

	audit.Status = "success"
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

// ValidateExtraVars checks a JSON object string against a task schema.
// A nil schema accepts any input. The returned error is the raw validation
// error; callers wrap it with ErrSvcTaskSchemaValidation.
func ValidateExtraVars(schema map[string]interface{}, extraVars string) error {
	if schema == nil {
		return nil
	}

	schemaData, err := json.Marshal(schema)
	if err != nil {
		return err
	}

	s := new(spec.Schema)
	_ = json.Unmarshal(schemaData, s)

	input := map[string]interface{}{}

	// JSON data to validate
	_ = json.Unmarshal([]byte(extraVars), &input)

	// strfmt.Default is the registry of recognized formats
	return validate.AgainstSchema(s, input, strfmt.Default)
}

func (t *TaskServiceImpl) RunTask(meta models.AuditMetadata, username string, taskID uuid.UUID, extraVars string) (models.Job, error) {
	audit := t.auditService.InitialiseAuditLog(meta, "run", "tasks", taskID.String())
	defer func() { t.auditService.CreateAudit(audit) }()

	job, err := t.executeTask(username, taskID, extraVars)
	if err != nil {
		return models.Job{}, err
	}

	audit.Status = "success"
	return job, nil
}

func (t *TaskServiceImpl) ExecuteTask(username string, taskID uuid.UUID, extraVars string) (models.Job, error) {
	return t.executeTask(username, taskID, extraVars)
}

func (t *TaskServiceImpl) executeTask(username string, taskID uuid.UUID, extraVars string) (models.Job, error) {
	ctx := context.Background()
	var jobStatus models.Job

	task, err := t.getTaskByID(taskID)
	if err != nil {
		return jobStatus, err
	}
	taskName := task.Name

	if err := ValidateExtraVars(task.Schema, extraVars); err != nil {
		return models.Job{}, fmt.Errorf("task '%s' schema: %w, %s", taskName, ErrSvcTaskSchemaValidation, err)
	}

	runner, err := t.RunnerService.GetRunnerForExecution(task.RunnerID)
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
		task.ID,
		task.Name,
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

	storedJob, err := t.JobService.StoreJob(models.Job{
		Name:      jobName,
		TaskID:    task.ID,
		TaskName:  taskName,
		Owner:     username,
		Status:    "running",
		StartTime: job.CreationTimestamp.Format(time.UnixDate),
	})
	if err != nil {
		if delErr := helpers.DeleteJob(t.config.Kube, jobName); delErr != nil {
			fmt.Println(delErr)
		}
		if envSecret != "" {
			_ = helpers.DeleteSecret(t.config.Kube, envSecret)
		}
		if tokenSecret != "" {
			_ = helpers.DeleteSecret(t.config.Kube, tokenSecret)
		}
		return jobStatus, fmt.Errorf("task '%s' store job: %w", taskName, err)
	}

	if task.Synchronous {
		waitErr := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 20*time.Second, false,
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
		if waitErr != nil {
			fmt.Printf("job '%s' wait: %v\n", jobName, waitErr)
		}

		if err := t.JobService.RefreshJob(storedJob.ID); err != nil {
			return jobStatus, err
		}

		return t.JobService.GetJob(storedJob.ID)
	}

	return storedJob, nil
}
