package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"golang.org/x/exp/slices"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type TaskService interface {
	ListTasks([]string, models.TaskQueryParams) ([]*models.Task, error)
	GetTask(string) (*models.Task, error)
	CreateTask(models.Task) (*models.Task, error)
	UpdateTask(models.Task) (*models.Task, error)
	DeleteTask(string) error
	GetSchema(string) (map[string]interface{}, error)
	DeleteSchema(string) error
	UpdateSchema(string, map[string]interface{}) (map[string]interface{}, error)
}

type TaskServiceImpl struct {
	WebhookService WebhookService
	config         config.Config
}

func NewTaskService(ws WebhookService, config config.Config) TaskService {
	return &TaskServiceImpl{
		WebhookService: ws,
		config:         config,
	}
}

func (t *TaskServiceImpl) ListTasks(authList []string, params models.TaskQueryParams) ([]*models.Task, error) {
	var tasks []*models.Task

	if len(authList) == 0 {
		return tasks, nil
	}

	configMaps, err := helpers.ListConfigMaps(t.config.Kube)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch list of tasks: %w", err)
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
						return nil, fmt.Errorf("task '%s' schema: %w", configMap.Data["name"], err)
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

	return filtered, nil

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

func (t *TaskServiceImpl) CreateTask(task models.Task) (*models.Task, error) {
	var jsonData []byte
	err := helpers.ValidateK8sConfigMapName(task.Name)
	if err != nil {
		return nil, fmt.Errorf("task '%s': %w",
			task.Name,
			ErrSvcObjNameK8sConfMap)
	}
	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, err)
	}
	if runner.Data["image"] == "" {
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
		}

		err = ValidateSchema(jsonData)
		if err != nil {
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
		if k8sErrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("task '%s': %w", task.Name, ErrSvcObjExists)
		}
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}

	configuredTask, err := t.GetTask(task.Name)
	if err != nil {
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}
	return configuredTask, nil
}

func (t *TaskServiceImpl) UpdateTask(task models.Task) (*models.Task, error) {
	var jsonData []byte

	_, err := helpers.GetConfigMap(t.config.Kube, task.Name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': %w", task.Name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}

	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
		}
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, err)
	}

	if runner.Data["image"] == "" {
		return nil, fmt.Errorf("task '%s': runner '%s': %w", task.Name, task.Runner, ErrSvcTaskNoRunner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
		}

		err = ValidateSchema(jsonData)
		if err != nil {
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
		return nil, fmt.Errorf("task '%s': %w", task.Name, err)
	}

	configuredTask, err := t.GetTask(task.Name)
	if err != nil {
		return nil, fmt.Errorf("task '%s' schema: %w", task.Name, err)
	}
	return configuredTask, nil
}

func (t *TaskServiceImpl) DeleteTask(name string) error {
	res, err := t.WebhookService.ListTaskWebhooks(name)
	if len(res) != 0 {
		return fmt.Errorf("cannot delete task %s, please remove associated webhooks first: %w", name, ErrSvcObjInUse)
	}

	err = helpers.DeleteConfigMap(t.config.Kube, name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return fmt.Errorf("task '%s': %w", name, err)
	}

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

func (t *TaskServiceImpl) UpdateSchema(name string, schema map[string]interface{}) (map[string]interface{}, error) {
	task, err := helpers.GetConfigMap(t.config.Kube, name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("task '%s': %w", name, err)

	}
	if task.Data["runner"] == "" {
		return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("task '%s' schema: %w", name, err)
	}

	err = ValidateSchema(data)
	if err != nil {
		errString := strings.ReplaceAll(err.Error(), "\"", "'")
		return nil, fmt.Errorf("task '%s' schema: %w, %s", task.Name, ErrSvcTaskSchemaValidation, errString)
	}

	task.Data["schema"] = string(data)
	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, task.Data, "update")
	if err != nil {
		return nil, fmt.Errorf("task '%s' schema: %w", name, err)
	}

	return schema, nil
}

func (t *TaskServiceImpl) DeleteSchema(name string) error {
	task, err := t.GetTask(name)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if task.Schema == nil {
		return nil
	}

	// Parsing a models.Task into a map
	b, _ := json.Marshal(task)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "schema")
	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, data, "update")
	if err != nil {
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
