package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"golang.org/x/exp/slices"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type RunnerService interface {
	ListRunners([]string, models.RunnerQueryParams) ([]models.Runner, error)
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

func (r *RunnerServiceImpl) ListRunners(authList []string, params models.RunnerQueryParams) ([]models.Runner, error) {
	var runnersList []models.Runner

	if len(authList) == 0 {
		return runnersList, nil
	}

	configMaps, err := helpers.ListConfigMaps(r.config.Kube)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch list of runners: %w", err)
	}

	for _, configMap := range configMaps.Items {
		// TODO: we don't currently have a way to identify what is a Runner configmap so I'm checking if it has an Image field
		// This will be changed when runners will live in a separate namespace
		if configMap.Data["image"] != "" {
			cm := models.Runner{
				Name:   configMap.Data["name"],
				Image:  configMap.Data["image"],
				GitURL: configMap.Data["gitURL"],
				Branch: configMap.Data["branch"],
			}
			if authList[0] != "*" {
				if slices.Contains(authList, configMap.Data["name"]) {
					runnersList = append(runnersList, cm)
				}
				continue
			}
			runnersList = append(runnersList, cm)
		}
	}

	var filtered []models.Runner
	for _, runner := range runnersList {
		if params.Name != "" && !strings.Contains(strings.ToLower(runner.Name), strings.ToLower(params.Name)) {
			continue
		}
		filtered = append(filtered, runner)
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

func (r *RunnerServiceImpl) GetRunner(name string) (*models.Runner, error) {
	configMap, err := helpers.GetConfigMap(r.config.Kube, name)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("runner '%s': %w", name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("runner '%s': %w", name, err)
	}

	if configMap.Data["image"] == "" {
		return nil, fmt.Errorf("runner '%s': %w", name, ErrSvcObjNotFound)
	}

	var runnerData models.Runner
	b, _ := json.Marshal(configMap.Data)
	_ = json.Unmarshal(b, &runnerData)

	tokenObjName := name + "-token"
	token, err := r.GetSecret(tokenObjName)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("runner '%s' git repo token: %w", name, err)
		}
	} else {
		runnerData.Token = token["token"]
	}

	secretCleared, err := r.GetSecret(name)
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("runner '%s' secrets: %w", name, err)
		}
	} else {
		runnerData.Secret = secretCleared
	}

	return &runnerData, nil
}

func (r *RunnerServiceImpl) CreateRunner(runner models.Runner) (*models.Runner, error) {
	err := helpers.ValidateK8sConfigMapName(runner.Name)
	if err != nil {
		return nil, fmt.Errorf("runner '%s': %w",
			runner.Name,
			ErrSvcObjNameK8sConfMap)
	}

	b, _ := json.Marshal(runner)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "token")
	delete(data, "secret")

	if data["branch"] == "" {
		data["branch"] = "main"
	}

	_, err = helpers.CreateOrUpdateConfigMap(r.config.Kube, data, "create")
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("runner '%s': %w", runner.Name, ErrSvcObjExists)
		}
		return nil, fmt.Errorf("runner '%s': %w", runner.Name, err)
	}

	// runner contains two types of secrets: git repo token and custom secrets, to be stored
	// in separate k8s secrets. token will be stored under runner name + token, secrets as runner name.
	if runner.Token != "" {
		tokenObjName := runner.Name + "-token"
		token := make(map[string]string)
		token["token"] = runner.Token
		_, err = helpers.CreateOrUpdateSecret(r.config.Kube, tokenObjName, token, "create")

		if err != nil {
			return nil, fmt.Errorf("runner '%s' create git repo token: %w", runner.Name, err)
		}
	}

	if runner.Secret != nil {
		_, err = r.UpdateSecret(runner.Name, runner.Secret)

		if err != nil {
			return nil, fmt.Errorf("runner '%s' secrets update: %w", runner.Name, err)
		}
	}

	runnerData, err := r.GetRunner(runner.Name)
	if err != nil {
		return nil, fmt.Errorf("get runner '%s': %w", runner.Name, err)
	}
	return runnerData, err
}

func (r *RunnerServiceImpl) UpdateRunner(runner models.Runner) (*models.Runner, error) {
	_, err := helpers.GetConfigMap(r.config.Kube, runner.Name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("runner '%s': %w", runner.Name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("get runner '%s': %w", runner.Name, err)
	}

	b, _ := json.Marshal(runner)
	var data map[string]string
	_ = json.Unmarshal(b, &data)
	delete(data, "token")
	delete(data, "secret")

	_, err = helpers.CreateOrUpdateConfigMap(r.config.Kube, data, "update")
	if err != nil {
		return nil, fmt.Errorf("runner '%s': %w", runner.Name, err)
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
			if k8sErrors.IsNotFound(err) {
				operation = "create"
			} else {
				return nil, fmt.Errorf("runner '%s' create git repo token: %w", runner.Name, err)
			}
		}
		_, err := helpers.CreateOrUpdateSecret(r.config.Kube, tokenObjName, token, operation)
		if err != nil {
			return nil, fmt.Errorf("runner '%s' update git repo token: %w", runner.Name, err)
		}
	} else if runner.Token == "" {
		err = helpers.DeleteSecret(r.config.Kube, tokenObjName)
		if err != nil && !k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("runner '%s' delete git repo token: %w", runner.Name, err)
		}
	}

	if runner.Secret != nil {
		_, err = r.UpdateSecret(runner.Name, runner.Secret)

		if err != nil {
			return nil, fmt.Errorf("runner '%s' update secrets: %w", runner.Name, err)
		}
	}

	updatedRunner, err := r.GetRunner(runner.Name)
	if err != nil {
		return nil, fmt.Errorf("get runner '%s': %w", runner.Name, err)
	}
	return updatedRunner, nil
}

func (r *RunnerServiceImpl) DeleteRunner(name string) error {
	configMap, err := helpers.GetConfigMap(r.config.Kube, name)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return fmt.Errorf("runner '%s': %w", name, ErrSvcObjNotFound)
		}
		return fmt.Errorf("runner '%s': %w", name, err)
	}

	if configMap.Data["image"] == "" {
		return fmt.Errorf("runner '%s': %w", name, ErrSvcObjNotFound)
	}

	configMaps, err := helpers.ListConfigMaps(r.config.Kube)
	if err != nil {
		return err
	}

	// Cheching for tasks associated to the runner before deleting it.
	for _, configMap := range configMaps.Items {
		runnerName := configMap.Data["runner"]
		if runnerName == name {
			return fmt.Errorf("runner is bound to task: %s , delete that first: %w", configMap.Data["name"], ErrSvcObjInUse)
		}
	}
	err = helpers.DeleteConfigMap(r.config.Kube, name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return fmt.Errorf("runner '%s': %w", name, ErrSvcObjNotFound)
		}
		return fmt.Errorf("runner '%s': %w", name, err)
	}

	err = helpers.DeleteSecret(r.config.Kube, name)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("runner '%s' secrets: %w", name, err)
	}

	return nil
}

func (r *RunnerServiceImpl) GetAdminGroups(secretName string) (string, error) {
	secret, err := helpers.GetSecret(r.config.Kube, secretName)

	if err != nil {
		return "", fmt.Errorf("secret '%s': %w", secretName, err)
	}

	accessGroups := secret.Data["accessGroups"]

	return string(accessGroups), nil
}

func (r *RunnerServiceImpl) ListAllJobs() ([]models.Job, error) {
	jobs, err := helpers.ListJobs(r.config.Kube, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list all jobs: %w", err)
	}

	var jobsRet []models.Job
	for _, job := range jobs.Items {
		var jobRet models.Job
		jobRet.Name = job.Name
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
		return nil, fmt.Errorf("failed to get secret '%s': %w", name, err)
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
	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get secret '%s': %w", name, err)
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

		if v != "" && v != v2 {
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
			return secretCleaned, fmt.Errorf("failed to update secret '%s': %w", name, err)
		}

		for key := range secretNew.Data {
			secretCleaned[key] = "************"
		}
		return secretCleaned, nil
	} else {
		err := helpers.DeleteSecret(r.config.Kube, name)
		if err != nil {
			return secretCleaned, fmt.Errorf("failed to delete secret '%s': %w", name, err)
		}
		return secretCleaned, nil
	}
}

func (r *RunnerServiceImpl) DeleteSecret(name string) error {

	_, err := r.GetRunner(name)
	if err != nil {
		return fmt.Errorf("runner %s not found: %w", name, err)
	}

	err = helpers.DeleteSecret(r.config.Kube, name)
	if err != nil && !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secret '%s': %w", name, err)
	}

	return nil
}
