package services

import (
	"fmt"
	"kriten-core/config"
	"kriten-core/helpers"
	"kriten-core/models"
	"time"

	"strings"

	"github.com/go-errors/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

type JobService interface {
	ListJobs([]string) ([]models.Job, error)
	GetJob(string, string) (string, error)
	CreateJob(string, string, string) (string, string, error)
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

func (j *JobServiceImpl) GetJob(username string, taskID string) (string, error) {
	labelSelector := "job-name=" + taskID
	if username != "" {
		labelSelector = labelSelector + ",owner=" + username
	}

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return "", err
	}

	if len(pods.Items) == 0 {
		return "", errors.New("no pods found - check job ID")
	}

	var logs string
	for _, pod := range pods.Items {
		// TODO: this will only retrieve logs for now, can be extended if needed
		log, err := helpers.GetLogs(j.config.Kube, pod.Name)
		if err != nil {
			return "", err
		}
		logs = logs + log
	}

	return logs, nil
}

func (j *JobServiceImpl) CreateJob(username string, taskName string, extraVars string) (string, string, error) {
	task, err := helpers.GetConfigMap(j.config.Kube, taskName)
	if err != nil {
		return "", "", err
	}
	runnerName := task.Data["runner"]

	runner, err := helpers.GetConfigMap(j.config.Kube, runnerName)
	if err != nil {
		return "", "", err
	}
	runnerImage := runner.Data["image"]
	gitURL := runner.Data["gitURL"]

	secret, err := helpers.GetSecret(j.config.Kube, runnerName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return "", "", err
		}
	} else {
		gitToken := string(secret.Data["token"])
		if gitToken != "" {
			gitURL = strings.Replace(gitURL, "://", "://"+gitToken+":@", 1)
		}
	}

	jobID, err := helpers.CreateJob(j.config.Kube, taskName, runnerImage, username, extraVars, task.Data["command"], gitURL)

	if err != nil {
		return "", "", err
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

		ret, err := j.GetJob(username, jobID)
		return jobID, ret, err
	}

	return jobID, "", nil
}

func (j *JobServiceImpl) GetTaskConfigMap(name string) (map[string]string, error) {
	configMap, err := helpers.GetConfigMap(j.config.Kube, name)

	if err != nil {
		return nil, err
	}

	return configMap.Data, err
}
