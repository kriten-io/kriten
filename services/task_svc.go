package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"golang.org/x/exp/slices"
)

type TaskService interface {
	ListTasks([]string) ([]map[string]string, error)
	GetTask(string) (*models.Task, error)
	CreateTask(models.Task) (*models.Task, error)
	UpdateTask(models.Task) (*models.Task, error)
	DeleteTask(string) error
	GetSchema(string) (map[string]interface{}, error)
	DeleteSchema(string) error
	UpdateSchema(string, map[string]interface{}) (map[string]interface{}, error)
}

type TaskServiceImpl struct {
	config config.Config
}

func NewTaskService(config config.Config) TaskService {
	return &TaskServiceImpl{
		config: config,
	}
}

func (t *TaskServiceImpl) ListTasks(authList []string) ([]map[string]string, error) {
	var tasksList []map[string]string

	if len(authList) == 0 {
		return tasksList, nil
	}

	configMaps, err := helpers.ListConfigMaps(t.config.Kube)
	if err != nil {
		return nil, err
	}

	for _, configMap := range configMaps.Items {
		runnerName := configMap.Data["runner"]
		if runnerName != "" {
			if authList[0] == "*" || slices.Contains(authList, configMap.Data["name"]) {
				delete(configMap.Data, "synchronous")
				// allow returning schema in the list of tasks to accomodate frontend
				// delete(configMap.Data, "schema")
				tasksList = append(tasksList, configMap.Data)
			}
		}
	}

	return tasksList, nil
}

func (t *TaskServiceImpl) GetTask(name string) (*models.Task, error) {
	var taskData models.Task
	configMap, err := helpers.GetConfigMap(t.config.Kube, name)
	if err != nil {
		return nil, err
	}
	if configMap.Data["runner"] == "" {
		return nil, fmt.Errorf("task %s not found", name)
	}

	// TODO: this is a temporary solution to return synchronous as a boolean
	b, _ := json.Marshal(configMap.Data)

	_ = json.Unmarshal(b, &taskData)
	taskData.Synchronous, _ = strconv.ParseBool(configMap.Data["synchronous"])

	if configMap.Data["schema"] != "" {
		var jsonData map[string]interface{}
		err = json.Unmarshal([]byte(configMap.Data["schema"]), &jsonData)
		if err != nil {
			return nil, err
		}
		taskData.Schema = jsonData
	}

	return &taskData, nil
}

func (t *TaskServiceImpl) CreateTask(task models.Task) (*models.Task, error) {
	var jsonData []byte

	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil || runner.Data["image"] == "" {
		return nil, fmt.Errorf("error retrieving runner %s, please specify an existing runner", task.Runner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			return nil, err
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	configuredTask, err := t.GetTask(task.Name)
	if err != nil {
		return nil, err
	}
	return configuredTask, err
}

func (t *TaskServiceImpl) UpdateTask(task models.Task) (*models.Task, error) {
	var jsonData []byte

	_, err := helpers.GetConfigMap(t.config.Kube, task.Name)
	if err != nil {
		return nil, err
	}

	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil || runner.Data["image"] == "" {
		return nil, fmt.Errorf("error retrieving runner %s, please specify an existing runner", task.Runner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			return nil, err
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	configuredTask, err := t.GetTask(task.Name)
	if err != nil {
		return nil, err
	}
	return configuredTask, err

}

func (t *TaskServiceImpl) DeleteTask(name string) error {
	err := helpers.DeleteConfigMap(t.config.Kube, name)
	if err != nil {
		return err
	}

	return nil
}

func (t *TaskServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
	var data map[string]interface{}

	configMap, err := helpers.GetConfigMap(t.config.Kube, name)
	if err != nil {
		return nil, err
	}
	if configMap.Data["runner"] == "" {
		return nil, fmt.Errorf("task %s not found", name)
	}

	if configMap.Data["schema"] != "" {
		err = json.Unmarshal([]byte(configMap.Data["schema"]), &data)
		if err != nil {
			return nil, err
		}

	}

	return data, nil
}

func (t *TaskServiceImpl) UpdateSchema(taskName string, schema map[string]interface{}) (map[string]interface{}, error) {
	task, err := helpers.GetConfigMap(t.config.Kube, taskName)
	if err != nil {
		return nil, err
	}
	if task.Data["runner"] == "" {
		return nil, fmt.Errorf("task %s not found", taskName)
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}

	err = ValidateSchema(data)
	if err != nil {
		return nil, err
	}

	task.Data["schema"] = string(data)
	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, task.Data, "update")
	if err != nil {
		return nil, err
	}

	return schema, nil
}

func (t *TaskServiceImpl) DeleteSchema(name string) error {
	task, err := t.GetTask(name)
	if err != nil {
		return err
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
		return err
	}

	return nil
}

func ValidateSchema(schema []byte) error {
	input, err := os.ReadFile("spec.json")
	if err != nil {
		log.Println(err)
		return err
	}

	output := bytes.Replace(input, []byte("\"%schema%\""), schema, -1)
	doc, err := loads.Analyzed(output, "2.0")
	if err != nil {
		log.Printf("error while loading spec: %v\n", err)
		return err
	}

	validate.SetContinueOnErrors(true)       // Set global options
	err = validate.Spec(doc, strfmt.Default) // Validates spec with default Swagger 2.0 format definitions

	if err != nil {
		log.Printf("This spec has some validation errors: %v\n", err)
		return err
	}

	return nil
}
