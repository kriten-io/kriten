package services

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"log"

	"encoding/json"
	"strings"

	"github.com/go-errors/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type CronJobService interface {
	ListCronJobs([]string) ([]models.CronJob, error)
	GetCronJob(string, string) (models.CronJob, error)
	CreateCronJob(string, string, string) (models.CronJob, error)
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
		log.Println(job)
		var jobRet models.CronJob
		jobRet.ID = job.Name
		jobRet.Owner = job.Labels["owner"]
		jobsList = append(jobsList, jobRet)
	}

	return jobsList, nil
}

func (j *CronJobServiceImpl) GetCronJob(username string, jobID string) (models.CronJob, error) {
	var jobStatus models.CronJob

	labelSelector := "job-name=" + jobID
	if username != "" {
		labelSelector = labelSelector + ",owner=" + username
	}

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return jobStatus, err
	}

	if len(pods.Items) == 0 {
		return jobStatus, errors.New("no pods found - check job ID")
	}

	job, err := helpers.GetCronJob(j.config.Kube, jobID)

	if err != nil {
		return jobStatus, err
	}

	jobStatus.ID = job.Name
	jobStatus.Owner = job.Labels["owner"]

	var logs string
	for _, pod := range pods.Items {
		// TODO: this will only retrieve logs for now, can be extended if needed
		log, err := helpers.GetLogs(j.config.Kube, pod.Name)
		if err != nil {
			return jobStatus, err
		}
		logs = logs + log

	}

	jobStatus.Stdout = logs

	if jobStatus.Stdout != "" {
		json_byte, _ := findDelimitedString(jobStatus.Stdout)

		if json_byte != nil {
			// ^JSON delimited text found in the log

			replacer := strings.NewReplacer("\n", "", "\\", "")
			json_string := replacer.Replace(string(json_byte))

			if err := json.Unmarshal([]byte(json_string), &jobStatus.JsonData); err != nil {
				jobStatus.JsonData = map[string]interface{}{"error": "failed to parse JSON"}
				return jobStatus, nil
			}
		}
	}

	return jobStatus, nil
}

func (j *CronJobServiceImpl) CreateCronJob(username string, taskName string, extraVars string) (models.CronJob, error) {
	var jobStatus models.CronJob

	task, err := helpers.GetConfigMap(j.config.Kube, taskName)
	if err != nil {
		return jobStatus, err
	}
	runnerName := task.Data["runner"]

	if task.Data["schema"] != "" {
		schema := new(spec.Schema)
		_ = json.Unmarshal([]byte(task.Data["schema"]), schema)

		input := map[string]interface{}{}

		// JSON data to validate
		_ = json.Unmarshal([]byte(extraVars), &input)

		// strfmt.Default is the registry of recognized formats
		err = validate.AgainstSchema(schema, input, strfmt.Default)
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

	jobID, err := helpers.CreateCronJob(j.config.Kube, taskName, runnerImage, username, extraVars, task.Data["command"], gitURL, gitBranch)

	jobStatus.ID = jobID

	if err != nil {
		return jobStatus, err
	}

	return jobStatus, nil
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
