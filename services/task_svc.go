package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"log"
	"os"
	"strconv"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"golang.org/x/exp/slices"

	"k8s.io/apimachinery/pkg/api/errors"
)

type TaskService interface {
	ListTasks([]string) ([]map[string]string, error)
	GetTask(string) (map[string]interface{}, map[string]string, error)
	CreateTask(models.Task) (map[string]interface{}, map[string]string, error)
	UpdateTask(models.Task) (map[string]interface{}, map[string]string, error)
	DeleteTask(string) error
	GetSchema(string) (map[string]interface{}, error)
	DeleteSchema(string) error
	UpdateSchema(string, map[string]interface{}) (map[string]interface{}, error)
	GetSecret(string) (map[string]string, error)
	UpdateSecret(string, map[string]string) (map[string]string, error)
	DeleteSecret(string) error
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
				delete(configMap.Data, "schema")
				tasksList = append(tasksList, configMap.Data)
			}
		}
	}

	return tasksList, nil
}

func (t *TaskServiceImpl) GetTask(name string) (map[string]interface{}, map[string]string, error) {
	configMap, err := helpers.GetConfigMap(t.config.Kube, name)
	if err != nil {
		return nil, nil, err
	}
	if configMap.Data["runner"] == "" {
		return nil, nil, fmt.Errorf("task %s not found", name)
	}

	// TODO: this is a temporary solution to return synchronous as a boolean
	b, _ := json.Marshal(configMap.Data)
	var data map[string]interface{}
	_ = json.Unmarshal(b, &data)
	data["synchronous"], _ = strconv.ParseBool(configMap.Data["synchronous"])

	if configMap.Data["schema"] != "" {
		var jsonData map[string]interface{}
		err = json.Unmarshal([]byte(configMap.Data["schema"]), &jsonData)
		if err != nil {
			return nil, nil, err
		}
		data["schema"] = jsonData
	}

	secretCleared, err := t.GetSecret(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return data, nil, nil
		}
		return data, nil, err
	}

	return data, secretCleared, nil
}

func (t *TaskServiceImpl) CreateTask(task models.Task) (map[string]interface{}, map[string]string, error) {
	var jsonData []byte
	var configuredTask map[string]interface{}
	var secretCleared map[string]string

	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil || runner.Data["image"] == "" {
		return nil, nil, fmt.Errorf("error retrieving runner %s, please specify an existing runner", task.Runner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			return nil, nil, err
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			return nil, nil, err
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
		return nil, nil, err
	}

	if task.Secret != nil {
		_, err = t.UpdateSecret(task.Name, task.Secret)

		if err != nil {
			return nil, nil, err
		}
	}

	configuredTask, secretCleared, err = t.GetTask(task.Name)
	if err != nil {
		return nil, nil, err
	}
	return configuredTask, secretCleared, err
}

func (t *TaskServiceImpl) UpdateTask(task models.Task) (map[string]interface{}, map[string]string, error) {
	var jsonData []byte
	var configuredTask map[string]interface{}
	var secretCleared map[string]string

	_, err := helpers.GetConfigMap(t.config.Kube, task.Name)
	if err != nil {
		return nil, nil, err
	}

	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil || runner.Data["image"] == "" {
		return nil, nil, fmt.Errorf("error retrieving runner %s, please specify an existing runner", task.Runner)
	}

	if task.Schema != nil {
		jsonData, err = json.Marshal(task.Schema)
		if err != nil {
			return nil, nil, err
		}

		err = ValidateSchema(jsonData)
		if err != nil {
			return nil, nil, err
		}
	}

	// Parsing a models.Task into a map
	b, _ := json.Marshal(task)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	data["synchronous"] = strconv.FormatBool(task.Synchronous)
	data["schema"] = string(jsonData)
	delete(data, "secret")

	_, err = helpers.CreateOrUpdateConfigMap(t.config.Kube, data, "update")
	if err != nil {
		return nil, nil, err
	}

	//var secret *v1.Secret
	if task.Secret != nil {

		_, err = t.UpdateSecret(task.Name, task.Secret)

		if err != nil {
			return nil, nil, err
		}

	} else {
		err = helpers.DeleteSecret(t.config.Kube, task.Name)
		if err != nil && !errors.IsNotFound(err) {
			return nil, nil, err
		}
	}

	configuredTask, secretCleared, err = t.GetTask(task.Name)
	if err != nil {
		return nil, nil, err
	}
	return configuredTask, secretCleared, err

}

func (t *TaskServiceImpl) DeleteTask(name string) error {
	err := helpers.DeleteConfigMap(t.config.Kube, name)
	if err != nil {
		return err
	}

	err = helpers.DeleteSecret(t.config.Kube, name)
	if err != nil && !errors.IsNotFound(err) {
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
	task, _, err := t.GetTask(name)
	if err != nil {
		return err
	}

	log.Println(task)
	log.Println(task["schema"])
	if task["schema"] == "" {
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

func (t *TaskServiceImpl) GetSecret(name string) (map[string]string, error) {
	secretCleaned := make(map[string]string)
	secret, err := helpers.GetSecret(t.config.Kube, name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	} else if errors.IsNotFound(err) {
		return nil, nil
	}

	for key := range secret.Data {
		secretCleaned[key] = "************"

	}

	return secretCleaned, nil
}

func (t *TaskServiceImpl) UpdateSecret(name string, secret map[string]string) (map[string]string, error) {
	secretCleaned := make(map[string]string)
	secretCurrent := make(map[string]string)
	var operation string

	secretObj, err := helpers.GetSecret(t.config.Kube, name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	// converting k8s secret from v1.Secret into map[string]string

	if secretObj != nil {

		for k, v := range secretObj.Data {
			secretCurrent[k] = string(v)
		}

		operation = "update"

	} else {
		operation = "create"
	}

	for k, v := range secret {

		v2, ok := secretCurrent[k]

		if v != "" || v != v2 {
			if v != "************" {
				secretCurrent[k] = v
			}

		} else if v == "" && ok {
			delete(secretCurrent, k)
		}
	}

	if len(secretCurrent) != 0 {
		secretNew, err := helpers.CreateOrUpdateSecret(t.config.Kube, name, secretCurrent, operation)
		if err != nil {
			return secretCleaned, err
		}

		for key := range secretNew.Data {
			secretCleaned[key] = "*************"

		}
		return secretCleaned, nil

	} else {
		err := helpers.DeleteSecret(t.config.Kube, name)
		if err != nil {
			return secretCleaned, err
		}
		return secretCleaned, nil
	}
}

func (t *TaskServiceImpl) DeleteSecret(name string) error {

	err := helpers.DeleteSecret(t.config.Kube, name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
