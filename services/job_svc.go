package services

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"log"
	"time"

	"encoding/json"
	"strings"

	"github.com/go-errors/errors"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

type JobService interface {
	ListJobs([]string) ([]models.Job, error)
	GetJob(string, string) (models.Job, error)
	GetLog(string, string) (string, error)
	CreateJob(string, string, string) (models.Job, error)
	GetSchema(string) (map[string]interface{}, error)
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

func (j *JobServiceImpl) GetJob(username string, jobID string) (models.Job, error) {
	var jobStatus models.Job

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

	for i, _ := range pods.Items {
		for c, _ := range pods.Items[i].Status.InitContainerStatuses {
			switch {
			case pods.Items[i].Status.InitContainerStatuses[c].State.Terminated.Reason == "ImagePullBackOff":
				return jobStatus, errors.New("failed to pull init container image from container registry.")
			case pods.Items[i].Status.InitContainerStatuses[c].State.Terminated.Reason == "Error":
				return jobStatus, errors.New("failed to clone repo: wrong repo url or incorrect credentials.")
			}
		}
		for c, _ := range pods.Items[i].Status.InitContainerStatuses {
			if pods.Items[i].Status.InitContainerStatuses[c].State.Terminated.Reason == "ImagePullBackOff" {
				return jobStatus, errors.New("failed to pull application container image from container registry.")
			}
		}
	}

	job, err := helpers.GetJob(j.config.Kube, jobID)

	if err != nil {
		return jobStatus, err
	}

	jobStatus.ID = job.Name
	jobStatus.Owner = job.Labels["owner"]
	jobStatus.StartTime = job.Status.StartTime.Format(time.UnixDate)
	if job.Status.CompletionTime != nil {
		jobStatus.CompletionTime = job.Status.CompletionTime.Format(time.UnixDate)
	}
	jobStatus.Failed = job.Status.Failed
	jobStatus.Completed = job.Status.Succeeded

	jobLog, err := j.GetLog(username, jobID)
	if err != nil {
		jobStatus.Stdout += fmt.Sprintf("failed to read logs from containers: %v", err)
	} else {
		jobStatus.Stdout += jobLog
	}

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

func (j *JobServiceImpl) GetLog(username string, jobID string) (string, error) {
	var logs string

	labelSelector := "job-name=" + jobID
	if username != "" {
		labelSelector = labelSelector + ",owner=" + username
	}

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return logs, err
	}

	if len(pods.Items) == 0 {
		return logs, errors.New("no pods found - check job ID")
	}

	for _, pod := range pods.Items {
		// TODO: this will only retrieve logs for now, can be extended if needed
		logs += "\n\n## init container logs\n"
		for c := range pod.Spec.InitContainers {
			jobLog, err := helpers.GetLogs(j.config.Kube, pod.Name, pod.Spec.InitContainers[c].Name)
			if err != nil {
				logs += fmt.Sprintf("error reading logs from init container: %v", err)
			} else {
				logs += jobLog
			}
		}
		// resetting jobLog to avoid duplications
		logs += "\n\n##application container logs \n"
		for c := range pod.Spec.Containers {
			jobLog, err := helpers.GetLogs(j.config.Kube, pod.Name, pod.Spec.Containers[c].Name)
			if err != nil {
				logs += fmt.Sprintf("error reading logs from application container: %v", err)
			} else {
				logs += jobLog
			}
		}
	}

	return logs, nil
}

func (j *JobServiceImpl) CreateJob(username string, taskName string, extraVars string) (models.Job, error) {
	var jobStatus models.Job

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
			return models.Job{}, err
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
	tokenObjName := runnerName + "-token"
	token, err := helpers.GetSecret(j.config.Kube, tokenObjName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return jobStatus, err
		}
	} else {
		gitToken := string(token.Data["token"])
		if gitToken != "" {
			gitURL = strings.Replace(gitURL, "://", "://"+gitToken+":@", 1)
		}
	}

	jobID, err := helpers.CreateJob(j.config.Kube, taskName, runnerName, runnerImage, username, extraVars, task.Data["command"], gitURL, gitBranch)

	jobStatus.ID = jobID

	if err != nil {
		return jobStatus, err
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
		return ret, err
	}

	return jobStatus, nil
}

func (j *JobServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
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
