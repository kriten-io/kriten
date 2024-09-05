package services

import (
	"encoding/json"
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"time"

	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/api/errors"
)

type RunnerService interface {
	ListRunners([]string) ([]map[string]string, error)
	GetRunner(string) (*models.Runner, error)
	CreateRunner(models.Runner) (*models.Runner, error)
	UpdateRunner(models.Runner) (*models.Runner, error)
	DeleteRunner(string) error
	GetAdminGroups(string) (string, error)
	ListAllJobs() ([]models.Job, error)
	GetSecret(string) (map[string]string, error)
	UpdateSecret(string, map[string]string) (map[string]string, error)
	DeleteSecret(string) error
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
		// TODO: we don't currently have a way to identify what is a Runner configmap so I'm checking if it has an Image field
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

func (r *RunnerServiceImpl) GetRunner(name string) (*models.Runner, error) {
	var runnerData models.Runner
	configMap, err := helpers.GetConfigMap(r.config.Kube, name)

	if err != nil {
		return &runnerData, err
	}

	if configMap.Data["image"] == "" {
		return nil, fmt.Errorf("runner %s not found", name)
	}

	b, _ := json.Marshal(configMap.Data)
	_ = json.Unmarshal(b, &runnerData)

	tokenObjName := name + "-token"
	token, err := r.GetSecret(tokenObjName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return &runnerData, err
		}
	} else {
		runnerData.Token = token["token"]
	}

	secretCleared, err := r.GetSecret(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return &runnerData, err
		}
	} else {
		runnerData.Secret = secretCleared
	}

	return &runnerData, nil
}

func (r *RunnerServiceImpl) CreateRunner(runner models.Runner) (*models.Runner, error) {
	b, _ := json.Marshal(runner)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "token")
	delete(data, "secret")

	if data["branch"] == "" {
		data["branch"] = "main"
	}

	_, err := helpers.CreateOrUpdateConfigMap(r.config.Kube, data, "create")
	if err != nil {
		return nil, err
	}

	// runner contains two types of secrets: git repo token and custom secrets, to be stored
	// in separate k8s secrets. token will be stored under runner name, secrets as runner name + secrets.
	if runner.Token != "" {
		tokenObjName := runner.Name + "-token"
		token := make(map[string]string)
		token["token"] = runner.Token
		_, err = helpers.CreateOrUpdateSecret(r.config.Kube, tokenObjName, token, "create")

		if err != nil {
			return nil, err
		}
	}

	if runner.Secret != nil {
		_, err = r.UpdateSecret(runner.Name, runner.Secret)

		if err != nil {
			return nil, err
		}
	}

	runnerData, err := r.GetRunner(runner.Name)
	return runnerData, err
}

func (r *RunnerServiceImpl) UpdateRunner(runner models.Runner) (*models.Runner, error) {
	_, err := helpers.GetConfigMap(r.config.Kube, runner.Name)
	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(runner)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "token")

	_, err = helpers.CreateOrUpdateConfigMap(r.config.Kube, data, "update")
	if err != nil {
		return nil, err
	}

	tokenObjName := runner.Name + "-token"
	if runner.Token != "" && runner.Token != "************" {
		token := make(map[string]string)
		token["token"] = runner.Token
		operation := "update"
		// default operation is 'update', try to get the Secret first: if it's not found we need to create it
		// e.g. Someone created a Task without a secret and is adding one with update
		_, err = r.GetSecret(tokenObjName)
		if err != nil {
			if errors.IsNotFound(err) {
				operation = "create"
			} else {
				return nil, err
			}
		}
		_, err := helpers.CreateOrUpdateSecret(r.config.Kube, tokenObjName, token, operation)
		if err != nil {
			return nil, err
		}
	} else if runner.Token == "" {
		err = helpers.DeleteSecret(r.config.Kube, tokenObjName)
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
	}

	if runner.Secret != nil {
		_, err = r.UpdateSecret(runner.Name, runner.Secret)

		if err != nil {
			return nil, err
		}

	}

	updatedRunner, err := r.GetRunner(runner.Name)
	if err != nil {
		return nil, err
	}
	return updatedRunner, err

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

func (r *RunnerServiceImpl) GetSecret(name string) (map[string]string, error) {
	secretCleaned := make(map[string]string)
	secret, err := helpers.GetSecret(r.config.Kube, name)

	if err != nil {
		return nil, err
	}

	for key := range secret.Data {
		secretCleaned[key] = "************"

	}
	return secretCleaned, nil
}

func (r *RunnerServiceImpl) UpdateSecret(name string, secret map[string]string) (map[string]string, error) {
	secretCleaned := make(map[string]string)
	secretCurrent := make(map[string]string)
	var operation string

	secretObj, err := helpers.GetSecret(r.config.Kube, name)
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
		secretNew, err := helpers.CreateOrUpdateSecret(r.config.Kube, name, secretCurrent, operation)
		if err != nil {
			return secretCleaned, err
		}

		for key := range secretNew.Data {
			secretCleaned[key] = "*************"

		}
		return secretCleaned, nil

	} else {
		err := helpers.DeleteSecret(r.config.Kube, name)
		if err != nil {
			return secretCleaned, err
		}
		return secretCleaned, nil
	}
}

func (r *RunnerServiceImpl) DeleteSecret(name string) error {
	err := helpers.DeleteSecret(r.config.Kube, name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
