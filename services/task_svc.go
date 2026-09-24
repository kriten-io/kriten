package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
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

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/util/wait"
)

type TaskService interface {
	ListTasks([]string, models.TaskQueryParams) ([]*models.Task, int, error)
	GetTask(string) (*models.Task, error)
	CreateTask(models.Actor, models.Task) (*models.Task, error)
	UpdateTask(models.Actor, models.Task) (*models.Task, error)
	DeleteTask(models.Actor, string) error
	GetSchema(string) (map[string]interface{}, error)
	DeleteSchema(models.Actor, string) error
	UpdateSchema(models.Actor, string, map[string]interface{}) (map[string]interface{}, error)
	RunTask(models.Actor, string, string) (models.Job, error)
}

const tasksCategory = "tasks"

type TaskServiceImpl struct {
	WebhookService WebhookService
	JobService     JobService
	config         config.Config
	audit          AuditService
}

func NewTaskService(ws WebhookService, config config.Config, js JobService, als AuditService) TaskService {
	return &TaskServiceImpl{
		WebhookService: ws,
		config:         config,
		JobService:     js,
		audit:          als,
	}
}

func (t *TaskServiceImpl) ListTasks(authList []string, params models.TaskQueryParams) ([]*models.Task, int, error) {
	var tasks []*models.Task

	if len(authList) == 0 {
		return tasks, 0, nil
	}

	configMaps, err := helpers.ListConfigMaps(t.config.Kube)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch list of tasks: %w", err)
	}

	for _, configMap := range configMaps.Items {
		runnerName := configMap.Data["runner"]
		if runnerName != "" {
			if authList[0] == "*" || slices.Contains(authList, configMap.Data["name"]) {
				var taskData *models.Task
				b, _ := json.Marshal(configMap.Data)

				_ = json.Unmarshal(b, &taskData)
				taskData.Synchronous, _ = strconv.ParseBool(configMap.Data["synchronous"])
				if configMap.Data["schema"] != "" {
					var jsonData map[string]interface{}
					err = json.Unmarshal([]byte(configMap.Data["schema"]), &jsonData)
					if err != nil {
						return nil, 0, fmt.Errorf("task '%s' schema: %w", configMap.Data["name"], err)
					}
					taskData.Schema = jsonData
				}
				tasks = append(tasks, taskData)
			}
		}

	}
	var filtered []*models.Task
	for _, task := range tasks {
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
	var taskData models.Task
	configMap, err := helpers.GetConfigMap(t.config.Kube, name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", name, err)
	}
	if configMap.Data["runner"] == "" {
		return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
	}

	// TODO: this is a temporary solution to return synchronous as a boolean
	b, _ := json.Marshal(configMap.Data)

	_ = json.Unmarshal(b, &taskData)
	taskData.Synchronous, _ = strconv.ParseBool(configMap.Data["synchronous"])

	if configMap.Data["schema"] != "" {
		var jsonData map[string]interface{}
		err = json.Unmarshal([]byte(configMap.Data["schema"]), &jsonData)
		if err != nil {
			return nil, fmt.Errorf("task '%s' schema: %w", name, err)
		}
		taskData.Schema = jsonData
	}

	return &taskData, nil
}

func (t *TaskServiceImpl) CreateTask(actor models.Actor, task models.Task) (*models.Task, error) {
	audit := t.audit.NewAuditLog(actor, "create", tasksCategory, task.Name)

	var jsonData []byte
	err := helpers.ValidateK8sConfigMapName(task.Name)
	if err != nil {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s': %w",
			task.Name,
			ErrSvcObjNameK8sConfMap)
	}
	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil {
		t.audit.CreateAudit(audit)
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, err)
	}
	if runner.Data["image"] == "" {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			t.audit.CreateAudit(audit)
			return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			t.audit.CreateAudit(audit)
			errString := strings.ReplaceAll(err.Error(), "\"", "'")
			return nil, fmt.Errorf("task '%s' schema: %w, %s", task.Name, ErrSvcTaskSchemaValidation, errString)
		}
	}

	// Parsing a models.Task into a map
	b, _ := json.Marshal(task)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	data["synchronous"] = strconv.FormatBool(task.Synchronous)
	data["schema"] = string(jsonData)
	delete(data, "secret")

	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, data, "create")
	if err != nil {
		t.audit.CreateAudit(audit)
		if k8sErrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("task '%s': %w", task.Name, ErrSvcObjExists)
		}
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}

	configuredTask, err := t.GetTask(task.Name)
	if err != nil {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}
	audit.Status = "success"
	t.audit.CreateAudit(audit)
	return configuredTask, nil
}

func (t *TaskServiceImpl) UpdateTask(actor models.Actor, task models.Task) (*models.Task, error) {
	audit := t.audit.NewAuditLog(actor, "update", tasksCategory, task.Name)

	var jsonData []byte

	_, err := helpers.GetConfigMap(t.config.Kube, task.Name)
	if err != nil {
		t.audit.CreateAudit(audit)
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': %w", task.Name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}

	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil {
		t.audit.CreateAudit(audit)
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, err)
	}

	if runner.Data["image"] == "" {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			t.audit.CreateAudit(audit)
			return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			t.audit.CreateAudit(audit)
			errString := strings.ReplaceAll(err.Error(), "\"", "'")
			return nil, fmt.Errorf("task '%s' schema: %w, %s", task.Name, ErrSvcTaskSchemaValidation, errString)
		}
	}

	// Parsing a models.Task into a map
	b, _ := json.Marshal(task)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	data["synchronous"] = strconv.FormatBool(task.Synchronous)
	data["schema"] = string(jsonData)

	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, data, "update")
	if err != nil {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}

	configuredTask, err := t.GetTask(task.Name)
	if err != nil {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
	}
	audit.Status = "success"
	t.audit.CreateAudit(audit)
	return configuredTask, nil
}

func (t *TaskServiceImpl) DeleteTask(actor models.Actor, name string) error {
	audit := t.audit.NewAuditLog(actor, "delete", tasksCategory, name)

	res, err := t.WebhookService.ListTaskWebhooks(name)
	if len(res) != 0 {
		t.audit.CreateAudit(audit)
		return fmt.Errorf("cannot delete task %s, please remove associated webhooks first: %w", name, ErrSvcObjInUse)
	}

	err = helpers.DeleteConfigMap(t.config.Kube, name)
	if err != nil {
		t.audit.CreateAudit(audit)
		if k8sErrors.IsNotFound(err) {
			return fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return fmt.Errorf("task '%s': %w", name, err)
	}

	audit.Status = "success"
	t.audit.CreateAudit(audit)
	return nil
}

func (t *TaskServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
	var data map[string]any

	configMap, err := helpers.GetConfigMap(t.config.Kube, name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", name, err)
	}
	if configMap.Data["runner"] == "" {
		return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
	}

	if configMap.Data["schema"] != "" {
		err = json.Unmarshal([]byte(configMap.Data["schema"]), &data)
		if err != nil {
			return nil, fmt.Errorf("task '%s' schema: %w", name, err)
		}
	}

	return data, nil
}

func (t *TaskServiceImpl) UpdateSchema(actor models.Actor, name string, schema map[string]interface{}) (map[string]interface{}, error) {
	audit := t.audit.NewAuditLog(actor, "update_schema", tasksCategory, name)

	task, err := helpers.GetConfigMap(t.config.Kube, name)
	if err != nil {
		t.audit.CreateAudit(audit)
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", name, err)

	}
	if task.Data["runner"] == "" {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s' schema: %w", name, err)
	}

	err = ValidateSchema(data)
	if err != nil {
		t.audit.CreateAudit(audit)
		errString := strings.ReplaceAll(err.Error(), "\"", "'")
		return nil, fmt.Errorf("task '%s' schema: %w, %s", task.Name, ErrSvcTaskSchemaValidation, errString)
	}

	task.Data["schema"] = string(data)
	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, task.Data, "update")
	if err != nil {
		t.audit.CreateAudit(audit)
		return nil, fmt.Errorf("task '%s' schema: %w", name, err)
	}

	audit.Status = "success"
	t.audit.CreateAudit(audit)
	return schema, nil
}

func (t *TaskServiceImpl) DeleteSchema(actor models.Actor, name string) error {
	audit := t.audit.NewAuditLog(actor, "delete_schema", tasksCategory, name)

	task, err := t.GetTask(name)
	if err != nil {
		t.audit.CreateAudit(audit)
		return fmt.Errorf("%w", err)
	}

	if task.Schema == nil {
		audit.Status = "success"
		t.audit.CreateAudit(audit)
		return nil
	}

	// Parsing a models.Task into a map
	b, _ := json.Marshal(task)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "schema")
	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, data, "update")
	if err != nil {
		t.audit.CreateAudit(audit)
		return fmt.Errorf("task '%s' schema: %w", name, err)
	}

	audit.Status = "success"
	t.audit.CreateAudit(audit)
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

func (t *TaskServiceImpl) RunTask(actor models.Actor, taskName string, extraVars string) (models.Job, error) {
	ctx := context.Background()
	var jobStatus models.Job
	audit := t.audit.NewAuditLog(actor, "run", tasksCategory, taskName)

	task, err := helpers.GetConfigMap(t.config.Kube, taskName)
	if err != nil {
		t.audit.CreateAudit(audit)
		return jobStatus, err
	}
	runnerName := task.Data["runner"]

	if task.Data["schema"] != "" {
		schema := new(spec.Schema)
		_ = json.Unmarshal([]byte(task.Data["schema"]), schema)

		input := map[string]interface{}{}

		// JSON data to validate
		_ = json.Unmarshal([]byte(extraVars), &input)

		// strfmt.Default is the registry of recognized formats
		err = validate.AgainstSchema(schema, input, strfmt.Default)
		if err != nil {
			t.audit.CreateAudit(audit)
			log.Printf("JSON does not validate against schema: %v", err)
			return models.Job{}, err
		}
	}

	runner, err := helpers.GetConfigMap(t.config.Kube, runnerName)
	if err != nil {
		t.audit.CreateAudit(audit)
		return jobStatus, err
	}
	runnerImage := runner.Data["image"]
	gitURL := runner.Data["gitURL"]
	gitBranch := runner.Data["branch"]

	if gitBranch == "" {
		gitBranch = "main"
	}
	tokenObjName := runnerName + "-token"
	token, err := helpers.GetSecret(t.config.Kube, tokenObjName)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			t.audit.CreateAudit(audit)
			return jobStatus, err
		}
	} else {
		gitToken := string(token.Data["token"])
		if gitToken != "" {
			gitURL = strings.Replace(gitURL, "://", "://"+gitToken+":@", 1)
		}
	}

	jobName, err := helpers.CreateJob(
		t.config.Kube,
		taskName,
		runnerName,
		runnerImage,
		actor.Username,
		extraVars,
		task.Data["command"],
		gitURL,
		gitBranch,
	)

	jobStatus.Name = jobName

	if err != nil {
		t.audit.CreateAudit(audit)
		return jobStatus, err
	}

	if task.Data["synchronous"] == "true" {
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

		if err != nil {
			t.audit.CreateAudit(audit)
			return models.Job{}, err
		}

		ret, err := t.JobService.GetJob(actor.Username, jobName)
		if err != nil {
			t.audit.CreateAudit(audit)
			return ret, err
		}

		audit.Status = "success"
		t.audit.CreateAudit(audit)
		return ret, nil
	}

	audit.Status = "success"
	t.audit.CreateAudit(audit)
	return jobStatus, nil
}
