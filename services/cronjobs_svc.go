package services

import (
	"fmt"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"encoding/json"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type CronJobService interface {
	ListCronJobs([]string) ([]models.CronJob, error)
	GetCronJob(string) (models.CronJob, error)
	CreateCronJob(models.Actor, models.CronJob) (models.CronJob, error)
	UpdateCronJob(models.Actor, models.CronJob) (models.CronJob, error)
	DeleteCronJob(models.Actor, string) error
	GetSchema(string) (map[string]interface{}, error)
}

const cronjobsCategory = "cronjobs"

type CronJobServiceImpl struct {
	config config.Config
	audit  AuditService
}

func NewCronJobService(config config.Config, als AuditService) CronJobService {
	return &CronJobServiceImpl{
		config: config,
		audit:  als,
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
		return nil, fmt.Errorf("cronjobs: %w", err)
	}

	for _, job := range jobs.Items {
		var data map[string]interface{}
		// This unmarshal is only used to fetch the extra vars, it doesn't look very reliable so it might need a rework
		containerEnv := job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env
		if len(containerEnv) > 0 {
			err = json.Unmarshal([]byte(containerEnv[0].Value), &data)
			if err != nil {
				return nil, fmt.Errorf("cronjobs: %w", err)
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
		if k8sErrors.IsNotFound(err) {
			return cronjob, fmt.Errorf("cronjob '%s': %w", name, ErrSvcObjNotFound)
		}
		return cronjob, err
	}

	var data map[string]interface{}
	// This unmarshal is only used to fetch the extra vars, it doesn't look very reliable so it might need a rework
	containerEnv := job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env
	if len(containerEnv) > 0 {
		err = json.Unmarshal([]byte(containerEnv[0].Value), &data)
		if err != nil {
			return cronjob, fmt.Errorf("cronjob '%s': %w", name, err)
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

func (j *CronJobServiceImpl) CreateCronJob(actor models.Actor, cronjob models.CronJob) (models.CronJob, error) {
	audit := j.audit.NewAuditLog(actor, "create", cronjobsCategory, cronjob.Task)
	cronjob.Owner = actor.Username

	runner, command, err := PreFlightChecks(j.config.Kube, cronjob)
	if err != nil {
		j.audit.CreateAudit(audit)
		return models.CronJob{}, fmt.Errorf("cronjob '%s': %w, %v", cronjob.Name, ErrSvcCronJobPrecheck, err)
	}

	_, err = helpers.CreateOrUpdateCronJob(j.config.Kube, cronjob, runner, command, "create")
	if err != nil {
		j.audit.CreateAudit(audit)
		return models.CronJob{}, fmt.Errorf("cronjob '%s': %w", cronjob.Name, err)
	}

	audit.Status = "success"
	j.audit.CreateAudit(audit)
	return cronjob, nil
}

func (j *CronJobServiceImpl) UpdateCronJob(actor models.Actor, cronjob models.CronJob) (models.CronJob, error) {
	audit := j.audit.NewAuditLog(actor, "update", cronjobsCategory, cronjob.Name)
	cronjob.Owner = actor.Username

	_, err := helpers.GetCronJob(j.config.Kube, cronjob.Name)
	if err != nil {
		j.audit.CreateAudit(audit)
		if k8sErrors.IsNotFound(err) {
			return models.CronJob{}, fmt.Errorf("cronjob '%s': %w", cronjob.Name, ErrSvcObjNotFound)
		}
		return models.CronJob{}, err
	}
	runner, command, err := PreFlightChecks(j.config.Kube, cronjob)
	if err != nil {
		j.audit.CreateAudit(audit)
		return models.CronJob{}, fmt.Errorf("cronjob '%s': %w, %v", cronjob.Name, ErrSvcCronJobPrecheck, err)
	}

	_, err = helpers.CreateOrUpdateCronJob(j.config.Kube, cronjob, runner, command, "update")
	if err != nil {
		j.audit.CreateAudit(audit)
		return models.CronJob{}, fmt.Errorf("cronjob '%s': %w", cronjob.Name, err)
	}
	audit.Status = "success"
	j.audit.CreateAudit(audit)
	return cronjob, err
}

func (j *CronJobServiceImpl) DeleteCronJob(actor models.Actor, name string) error {
	audit := j.audit.NewAuditLog(actor, "delete", cronjobsCategory, name)

	_, err := helpers.GetCronJob(j.config.Kube, name)
	if err != nil {
		j.audit.CreateAudit(audit)
		if k8sErrors.IsNotFound(err) {
			return fmt.Errorf("cronjob '%s': %w", name, ErrSvcObjNotFound)
		}
		return fmt.Errorf("failed to get cronjob '%s': %w", name, err)
	}

	err = helpers.DeleteCronJob(j.config.Kube, name)
	if err != nil {
		j.audit.CreateAudit(audit)
		return fmt.Errorf("failed to delete cronjob %s: %w", name, err)
	}

	audit.Status = "success"
	j.audit.CreateAudit(audit)
	return nil
}

func (j *CronJobServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
	var data map[string]interface{}

	configMap, err := helpers.GetConfigMap(j.config.Kube, name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("failed to get task '%s': %w", name, err)
	}
	if configMap.Data["runner"] == "" {
		return nil, fmt.Errorf("task '%s': %w", name, ErrSvcObjNotFound)
	}

	if configMap.Data["schema"] != "" {
		err = json.Unmarshal([]byte(configMap.Data["schema"]), &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse task '%s' schema: %w", name, err)
		}
	}

	return data, nil
}

func PreFlightChecks(kube config.KubeConfig, cronjob models.CronJob) (*corev1.ConfigMap, string, error) {
	task, err := helpers.GetConfigMap(kube, cronjob.Task)
	if err != nil {
		return nil, "", fmt.Errorf("task '%s' not found", cronjob.Task)
	}

	if task.Data["schema"] != "" {
		schema := new(spec.Schema)
		_ = json.Unmarshal([]byte(task.Data["schema"]), schema)

		// strfmt.Default is the registry of recognized formats
		err = validate.AgainstSchema(schema, cronjob.ExtraVars, strfmt.Default)
		if err != nil {
			return nil, "", fmt.Errorf("validation failed against schema: %v", err)
		}
	}

	runner, err := helpers.GetConfigMap(kube, task.Data["runner"])
	if err != nil {
		return nil, "", fmt.Errorf("runner '%s' not found: %v", task.Data["runner"], err)
	}

	if runner.Data["branch"] == "" {
		runner.Data["branch"] = "main"
	}

	secret, err := helpers.GetSecret(kube, task.Data["runner"]+"-token")
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return nil, "", fmt.Errorf("failed to fetch secrets: %v", err)
		}
	} else {
		gitToken := string(secret.Data["token"])
		if gitToken != "" {
			runner.Data["gitURL"] = strings.Replace(runner.Data["gitURL"], "://", "://"+gitToken+":@", 1)
		}
	}

	return runner, task.Data["command"], nil
}
