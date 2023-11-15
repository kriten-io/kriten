package services

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"time"

	"encoding/json"
	"strings"

	"github.com/go-errors/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

type JobService interface {
	ListJobs([]string) ([]models.Job, error)
	GetJob(string, string) (models.Job, map[string]interface{}, error)
	CreateJob(string, string, string) (string, models.Job, map[string]interface{}, error)
	GetTaskConfigMap(string) (map[string]string, error)
}

type JobServiceImpl struct {
	config config.Config
}

func NewJobService(config config.Config) JobService {
	return &JobServiceImpl{
		config: config,
	}
}

func findDelimitedString(str string) ([]byte, error) {
	delimiter := "^JSON"
	var match []byte
	index := strings.Index(str, delimiter)

	if index == -1 {
		return match, nil
	}

	index += len(delimiter)

	for {
		char := str[index]

		if strings.HasPrefix(str[index:index+len(delimiter)], delimiter) {
			break
		}

		match = append(match, char)
		index++

		if index+len(delimiter) >= len(str) {
			match = nil
			break
		}

	}

	return match, nil
}

func (j *JobServiceImpl) ListJobs(authList []string) ([]models.Job, error) {
	var jobsList []models.Job
	var labelSelector []string

	if len(authList) == 0 {
		return jobsList, nil
	}

	if authList[0] != "*" {
		for _, s := range authList {
			labelSelector = append(labelSelector, "task-name="+s)
		}
	}
	// labelSelector := "task-name=" + taskName
	// if username != "" {
	// 	labelSelector = labelSelector + ",owner=" + username
	// }

	jobs, err := helpers.ListJobs(j.config.Kube, labelSelector)
	if err != nil {
		return nil, err
	}

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
		jobsList = append(jobsList, jobRet)
	}

	return jobsList, nil
}

func (j *JobServiceImpl) GetJob(username string, jobID string) (models.Job, map[string]interface{}, error) {
	var jobStatus models.Job
	var json_data map[string]interface{}

	labelSelector := "job-name=" + jobID
	if username != "" {
		labelSelector = labelSelector + ",owner=" + username
	}

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return jobStatus, json_data, err
	}

	if len(pods.Items) == 0 {
		return jobStatus, json_data, errors.New("no pods found - check job ID")
	}

	job, err := helpers.GetJob(j.config.Kube, jobID)

	if err != nil {
		return jobStatus, json_data, err
	}

	jobStatus.ID = job.Name
	jobStatus.Owner = job.Labels["owner"]
	jobStatus.StartTime = job.Status.StartTime.Format(time.UnixDate)
	if job.Status.CompletionTime != nil {
		jobStatus.CompletionTime = job.Status.CompletionTime.Format(time.UnixDate)
	}
	jobStatus.Failed = job.Status.Failed
	jobStatus.Completed = job.Status.Succeeded

	var logs string
	for _, pod := range pods.Items {
		// TODO: this will only retrieve logs for now, can be extended if needed
		log, err := helpers.GetLogs(j.config.Kube, pod.Name)
		if err != nil {
			return jobStatus, json_data, err
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

			if err := json.Unmarshal([]byte(json_string), &json_data); err != nil {
				return jobStatus, json_data, errors.New("failed to decode JSON")
			}
		}
	}

	return jobStatus, json_data, nil
}

func (j *JobServiceImpl) CreateJob(username string, taskName string, extraVars string) (string, models.Job, map[string]interface{}, error) {
	var jobStatus models.Job
	var jsonData map[string]interface{}

	task, err := helpers.GetConfigMap(j.config.Kube, taskName)
	if err != nil {
		return "", jobStatus, jsonData, err
	}
	runnerName := task.Data["runner"]

	runner, err := helpers.GetConfigMap(j.config.Kube, runnerName)
	if err != nil {
		return "", jobStatus, jsonData, err
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
			return "", jobStatus, jsonData, err
		}
	} else {
		gitToken := string(secret.Data["token"])
		if gitToken != "" {
			gitURL = strings.Replace(gitURL, "://", "://"+gitToken+":@", 1)
		}
	}

	jobID, err := helpers.CreateJob(j.config.Kube, taskName, runnerImage, username, extraVars, task.Data["command"], gitURL, gitBranch)

	if err != nil {
		return "", jobStatus, jsonData, err
	}

	if task.Data["synchronous"] == "true" {
		_ = wait.Poll(100*time.Millisecond, 20*time.Second, func() (done bool, err error) {

			job, err := helpers.GetJob(j.config.Kube, jobID)

			if err != nil {
				fmt.Println(err)
				return false, err
			}

			if job.Status.Succeeded != 0 || job.Status.Failed != 0 {
				return true, nil
			}

			return false, nil
		})

		ret, jsonData, err := j.GetJob(username, jobID)
		return jobID, ret, jsonData, err
	}

	return jobID, jobStatus, jsonData, nil
}

func (j *JobServiceImpl) GetTaskConfigMap(name string) (map[string]string, error) {
	configMap, err := helpers.GetConfigMap(j.config.Kube, name)

	if err != nil {
		return nil, err
	}

	return configMap.Data, err
}
