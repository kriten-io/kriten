package services

import (
	"encoding/json"
	"fmt"
	"kriten-core/config"
	"kriten-core/helpers"
	"kriten-core/models"
	"time"

	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type RunnerService interface {
	ListRunners([]string) ([]map[string]string, error)
	GetRunner(string) (map[string]string, error)
	CreateRunner(models.Runner) (*v1.ConfigMap, error)
	UpdateRunner(models.Runner) (*v1.ConfigMap, error)
	DeleteRunner(string) error
	GetAdminGroups(string) (string, error)
	ListAllJobs() ([]models.Job, error)
}

type RunnerServiceImpl struct {
	config config.Config
}

func NewRunnerService(config config.Config) RunnerService {
	return &RunnerServiceImpl{
		config: config,
	}
}

func (r *RunnerServiceImpl) ListRunners(authList []string) ([]map[string]string, error) {
	var runnersList []map[string]string

	if len(authList) == 0 {
		return runnersList, nil
	}

	configMaps, err := helpers.ListConfigMaps(r.config.Kube)
	if err != nil {
		return nil, err
	}

	for _, configMap := range configMaps.Items {
		// TODO: we don't currently have a way of definying what is a Runner configmap so I'm checking if it has an Image field
		// This will be changed when runners will live in a separate namespace
		if configMap.Data["image"] != "" {
			if authList[0] != "*" {
				if slices.Contains(authList, configMap.Data["name"]) {
					runnersList = append(runnersList, configMap.Data)
				}
				continue
			}
			runnersList = append(runnersList, configMap.Data)
		}
	}

	return runnersList, nil
}

func (r *RunnerServiceImpl) GetRunner(name string) (map[string]string, error) {
	configMap, err := helpers.GetConfigMap(r.config.Kube, name)

	if err != nil {
		return nil, err
	}

	if configMap.Data["image"] == "" {
		return nil, fmt.Errorf("runner %s not found", name)
	}

	secret, err := helpers.GetSecret(r.config.Kube, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return configMap.Data, nil
		}
		return nil, err
	}
	configMap.Data["token"] = string(secret.Data["token"])

	return configMap.Data, nil
}

func (r *RunnerServiceImpl) CreateRunner(runner models.Runner) (*v1.ConfigMap, error) {
	b, _ := json.Marshal(runner)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "token")

	configMap, err := helpers.CreateOrUpdateConfigMap(r.config.Kube, data, "create")

	if runner.Token != "" {
		secret := make(map[string]string)
		secret["token"] = runner.Token
		_, err = helpers.CreateOrUpdateSecret(r.config.Kube, runner.Name, secret, "create")

		if err != nil {
			return nil, err
		}
	}

	return configMap, err
}

func (r *RunnerServiceImpl) UpdateRunner(runner models.Runner) (*v1.ConfigMap, error) {
	_, err := helpers.GetConfigMap(r.config.Kube, runner.Name)
	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(runner)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "token")

	configMap, err := helpers.CreateOrUpdateConfigMap(r.config.Kube, data, "update")
	if err != nil {
		return nil, err
	}

	if runner.Token != "" {
		secret := make(map[string]string)
		secret["token"] = runner.Token
		operation := "update"
		// default operation is 'update', try to get the Secret first: if it's not found we need to create it
		// e.g. Someone created a Task without a secret and is adding one with update
		_, err = helpers.GetSecret(r.config.Kube, runner.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				operation = "create"
			} else {
				return nil, err
			}
		}
		_, err := helpers.CreateOrUpdateSecret(r.config.Kube, runner.Name, secret, operation)
		if err != nil {
			return nil, err
		}
	} else {
		err = helpers.DeleteSecret(r.config.Kube, runner.Name)
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
	}

	return configMap, err
}

func (r *RunnerServiceImpl) DeleteRunner(name string) error {
	configMaps, err := helpers.ListConfigMaps(r.config.Kube)
	if err != nil {
		return err
	}

	// Cheching for tasks associated to the runner before deleting it.
	for _, configMap := range configMaps.Items {
		runnerName := configMap.Data["runner"]
		if runnerName == name {
			return fmt.Errorf("runner is bound with task: %s , please delete that first", configMap.Data["name"])
		}
	}
	err = helpers.DeleteConfigMap(r.config.Kube, name)

	if err != nil {
		return err
	}

	err = helpers.DeleteSecret(r.config.Kube, name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *RunnerServiceImpl) GetAdminGroups(secretName string) (string, error) {
	secret, err := helpers.GetSecret(r.config.Kube, secretName)

	if err != nil {
		return "", err
	}

	accessGroups := secret.Data["accessGroups"]

	return string(accessGroups), nil
}

func (r *RunnerServiceImpl) ListAllJobs() ([]models.Job, error) {
	jobs, err := helpers.ListJobs(r.config.Kube, nil)
	if err != nil {
		return nil, err
	}

	var jobsRet []models.Job
	for _, job := range jobs.Items {
		var jobRet models.Job
		jobRet.ID = job.Name
		jobRet.Owner = job.Labels["owner"]
		jobRet.StartTime = job.Status.StartTime.Format(time.UnixDate)
		if job.Status.CompletionTime != nil {
			jobRet.CompletionTime = job.Status.CompletionTime.Format(time.UnixDate)
		}
		jobRet.Failed = job.Status.Failed
		jobRet.Completed = job.Status.Succeeded
		jobsRet = append(jobsRet, jobRet)
	}

	return jobsRet, nil
}
