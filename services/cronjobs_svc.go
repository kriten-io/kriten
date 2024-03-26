package services

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"log"

	"encoding/json"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type CronJobService interface {
	ListCronJobs([]string) ([]models.CronJob, error)
	GetCronJob(string) (models.CronJob, error)
	CreateCronJob(models.CronJob) (models.CronJob, error)
	GetTaskConfigMap(string) (map[string]string, error)
	GetSchema(string) (map[string]interface{}, error)
}

type CronJobServiceImpl struct {
	config config.Config
}

func NewCronJobService(config config.Config) CronJobService {
	return &CronJobServiceImpl{
		config: config,
	}
}

func (j *CronJobServiceImpl) ListCronJobs(authList []string) ([]models.CronJob, error) {
	var jobsList []models.CronJob
	var labelSelector []string

	if len(authList) == 0 {
		return jobsList, nil
	}

	if authList[0] != "*" {
		for _, s := range authList {
			labelSelector = append(labelSelector, "task-name="+s)
		}
	}

	jobs, err := helpers.ListCronJobs(j.config.Kube, labelSelector)
	if err != nil {
		return nil, err
	}

	for _, job := range jobs.Items {
		var data map[string]interface{}
		// _ = json.Unmarshal(b, &data)
		err = json.Unmarshal([]byte(job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env[0].Value), &data)
		if err != nil {
			return nil, err
		}
		jobRet := models.CronJob{
			ID:        job.Name,
			Owner:     job.Spec.JobTemplate.Spec.Template.Labels["owner"],
			Task:      job.Spec.JobTemplate.Spec.Template.Labels["task-name"],
			Schedule:  job.Spec.Schedule,
			Disable:   *job.Spec.Suspend,
			ExtraVars: data,
		}
		jobsList = append(jobsList, jobRet)
	}

	return jobsList, nil
}

func (j *CronJobServiceImpl) GetCronJob(name string) (models.CronJob, error) {
	var cronjob models.CronJob

	job, err := helpers.GetCronJob(j.config.Kube, name)
	if err != nil {
		return cronjob, err
	}

	var data map[string]interface{}
	// _ = json.Unmarshal(b, &data)
	err = json.Unmarshal([]byte(job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env[0].Value), &data)
	if err != nil {
		return cronjob, err
	}
	cronjob = models.CronJob{
		ID:        job.Name,
		Owner:     job.Spec.JobTemplate.Spec.Template.Labels["owner"],
		Task:      job.Spec.JobTemplate.Spec.Template.Labels["task-name"],
		Schedule:  job.Spec.Schedule,
		Disable:   *job.Spec.Suspend,
		ExtraVars: data,
	}

	return cronjob, nil
}

func (j *CronJobServiceImpl) CreateCronJob(cronjob models.CronJob) (models.CronJob, error) {
	var jobStatus models.CronJob

	task, err := helpers.GetConfigMap(j.config.Kube, cronjob.Task)
	if err != nil {
		return jobStatus, err
	}
	runnerName := task.Data["runner"]

	if task.Data["schema"] != "" {
		schema := new(spec.Schema)
		_ = json.Unmarshal([]byte(task.Data["schema"]), schema)

		// strfmt.Default is the registry of recognized formats
		err = validate.AgainstSchema(schema, cronjob.ExtraVars, strfmt.Default)
		if err != nil {
			log.Printf("JSON does not validate against schema: %v", err)
			return models.CronJob{}, err
		}
	}

	runner, err := helpers.GetConfigMap(j.config.Kube, runnerName)
	if err != nil {
		return jobStatus, err
	}
	runnerImage := runner.Data["image"]
	gitURL := runner.Data["gitURL"]
	gitBranch := runner.Data["branch"]

	if gitBranch == "" {
		gitBranch = "main"
	}

	secret, err := helpers.GetSecret(j.config.Kube, runnerName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return jobStatus, err
		}
	} else {
		gitToken := string(secret.Data["token"])
		if gitToken != "" {
			gitURL = strings.Replace(gitURL, "://", "://"+gitToken+":@", 1)
		}
	}

	_, err = helpers.CreateCronJob(j.config.Kube, runnerImage, cronjob, task.Data["command"], gitURL, gitBranch)

	return cronjob, err
}

func (j *CronJobServiceImpl) GetTaskConfigMap(name string) (map[string]string, error) {
	configMap, err := helpers.GetConfigMap(j.config.Kube, name)

	if err != nil {
		return nil, err
	}

	return configMap.Data, err
}

func (j *CronJobServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
	var data map[string]interface{}

	configMap, err := helpers.GetConfigMap(j.config.Kube, name)
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
