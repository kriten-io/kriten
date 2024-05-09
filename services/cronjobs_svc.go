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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type CronJobService interface {
	ListCronJobs([]string) ([]models.CronJob, error)
	GetCronJob(string) (models.CronJob, error)
	CreateCronJob(models.CronJob) (models.CronJob, error)
	UpdateCronJob(models.CronJob) (models.CronJob, error)
	DeleteCronJob(string) error
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
		// This unmarshal is only used to fetch the extra vars, it doesn't look very reliable so it might need a rework
		containerEnv := job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env
		if len(containerEnv) > 0 {
			err = json.Unmarshal([]byte(containerEnv[0].Value), &data)
			if err != nil {
				return nil, err
			}
		}
		jobRet := models.CronJob{
			Name:      job.Name,
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
	// This unmarshal is only used to fetch the extra vars, it doesn't look very reliable so it might need a rework
	containerEnv := job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env
	if len(containerEnv) > 0 {
		err = json.Unmarshal([]byte(containerEnv[0].Value), &data)
		if err != nil {
			return cronjob, err
		}
	}
	cronjob = models.CronJob{
		Name:      job.Name,
		Owner:     job.Spec.JobTemplate.Spec.Template.Labels["owner"],
		Task:      job.Spec.JobTemplate.Spec.Template.Labels["task-name"],
		Schedule:  job.Spec.Schedule,
		Disable:   *job.Spec.Suspend,
		ExtraVars: data,
	}

	return cronjob, nil
}

func (j *CronJobServiceImpl) CreateCronJob(cronjob models.CronJob) (models.CronJob, error) {
	runner, command, err := PreFlightChecks(j.config.Kube, cronjob)

	_, err = helpers.CreateOrUpdateCronJob(j.config.Kube, cronjob, runner, command, "create")

	return cronjob, err
}

func (j *CronJobServiceImpl) UpdateCronJob(cronjob models.CronJob) (models.CronJob, error) {
	runner, command, err := PreFlightChecks(j.config.Kube, cronjob)

	_, err = helpers.CreateOrUpdateCronJob(j.config.Kube, cronjob, runner, command, "update")

	return cronjob, err
}

func (j *CronJobServiceImpl) DeleteCronJob(id string) error {
	err := helpers.DeleteCronJob(j.config.Kube, id)
	if err != nil {
		return err
	}

	return nil
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

func PreFlightChecks(kube config.KubeConfig, cronjob models.CronJob) (*corev1.ConfigMap, string, error) {
	task, err := helpers.GetConfigMap(kube, cronjob.Task)
	if err != nil {
		return nil, "", err
	}

	if task.Data["schema"] != "" {
		schema := new(spec.Schema)
		_ = json.Unmarshal([]byte(task.Data["schema"]), schema)

		// strfmt.Default is the registry of recognized formats
		err = validate.AgainstSchema(schema, cronjob.ExtraVars, strfmt.Default)
		if err != nil {
			log.Printf("JSON does not validate against schema: %v", err)
			return nil, "", err
		}
	}

	runner, err := helpers.GetConfigMap(kube, task.Data["runner"])
	if err != nil {
		return nil, "", err
	}

	if runner.Data["branch"] == "" {
		runner.Data["branch"] = "main"
	}

	secret, err := helpers.GetSecret(kube, task.Data["runner"])
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, "", err
		}
	} else {
		gitToken := string(secret.Data["token"])
		if gitToken != "" {
			runner.Data["gitURL"] = strings.Replace(runner.Data["gitURL"], "://", "://"+gitToken+":@", 1)
		}
	}

	return runner, task.Data["command"], nil

}
