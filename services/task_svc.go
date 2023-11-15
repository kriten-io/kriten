package services

import (
	"encoding/json"
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"strconv"

	"golang.org/x/exp/slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type TaskService interface {
	ListTasks([]string) ([]map[string]string, error)
	GetTask(string) (map[string]interface{}, map[string][]byte, error)
	CreateTask(models.Task) (*v1.ConfigMap, *v1.Secret, error)
	UpdateTask(models.Task) (*v1.ConfigMap, *v1.Secret, error)
	DeleteTask(name string) error
	GetRunnerGroups(string) (string, error)
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
			if authList[0] != "*" {
				if slices.Contains(authList, configMap.Data["name"]) {
					tasksList = append(tasksList, configMap.Data)
				}
				continue
			}
			tasksList = append(tasksList, configMap.Data)
		}
	}

	return tasksList, nil
}

func (t *TaskServiceImpl) GetTask(name string) (map[string]interface{}, map[string][]byte, error) {
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

	secret, err := helpers.GetSecret(t.config.Kube, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return data, nil, nil
		}
		return data, nil, err
	}

	return data, secret.Data, nil
}

func (t *TaskServiceImpl) CreateTask(task models.Task) (*v1.ConfigMap, *v1.Secret, error) {
	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil || runner.Data["image"] == "" {
		return nil, nil, fmt.Errorf("error retrieving runner %s, please specify an existing runner", task.Runner)
	}

	// Parsing a models.Task into a map
	b, _ := json.Marshal(task)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	data["synchronous"] = strconv.FormatBool(task.Synchronous)
	delete(data, "secret")

	configMap, err := helpers.CreateOrUpdateConfigMap(t.config.Kube, data, "create")
	if err != nil {
		return nil, nil, err
	}

	var secret *v1.Secret
	if task.Secret != nil {
		secret, err = helpers.CreateOrUpdateSecret(t.config.Kube, task.Name, task.Secret, "create")

		if err != nil {
			return configMap, nil, err
		}
	}

	return configMap, secret, err
}

func (t *TaskServiceImpl) UpdateTask(task models.Task) (*v1.ConfigMap, *v1.Secret, error) {
	_, err := helpers.GetConfigMap(t.config.Kube, task.Name)
	if err != nil {
		return nil, nil, err
	}

	runner, err := helpers.GetConfigMap(t.config.Kube, task.Runner)
	if err != nil || runner.Data["image"] == "" {
		return nil, nil, fmt.Errorf("error retrieving runner %s, please specify an existing runner", task.Runner)
	}

	// Parsing a models.Task into a map
	b, _ := json.Marshal(task)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	data["synchronous"] = strconv.FormatBool(task.Synchronous)
	delete(data, "secret")

	configMap, err := helpers.CreateOrUpdateConfigMap(t.config.Kube, data, "update")
	if err != nil {
		return nil, nil, err
	}

	var secret *v1.Secret
	if task.Secret != nil {
		operation := "update"
		// default operation is 'update', try to get the Secret first: if it's not found we need to create it
		// e.g. Someone created a Task without a secret and is adding one with update
		_, err = helpers.GetSecret(t.config.Kube, task.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				operation = "create"
			} else {
				return configMap, nil, err
			}
		}
		secret, err = helpers.CreateOrUpdateSecret(t.config.Kube, task.Name, task.Secret, operation)
		if err != nil {
			return configMap, nil, err
		}
	} else {
		err = helpers.DeleteSecret(t.config.Kube, task.Name)
		if err != nil && !errors.IsNotFound(err) {
			return configMap, nil, err
		}
	}

	return configMap, secret, err
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

func (t *TaskServiceImpl) GetRunnerGroups(runnerName string) (string, error) {
	configMap, err := helpers.GetConfigMap(t.config.Kube, runnerName)

	if err != nil {
		return "", err
	}

	accessGroups := configMap.Data["accessGroups"]
	return string(accessGroups), nil
}
