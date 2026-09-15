package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"encoding/json"
	"strings"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type JobService interface {
	ListJobs([]string, models.JobQueryParams) ([]models.Job, int, error)
	GetJob(string, string) (models.Job, error)
	GetLog(string, string) (string, error)
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

func (j *JobServiceImpl) ListJobs(authList []string, params models.JobQueryParams) ([]models.Job, int, error) {
	var jobsList []models.Job
	var labelSelector []string

	if len(authList) == 0 {
		return jobsList, 0, nil
	}

	if authList[0] != "*" {
		for _, s := range authList {
			labelSelector = append(labelSelector, "task-name="+s)
		}
	}

	jobs, err := helpers.ListJobs(j.config.Kube, labelSelector)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	if len(jobs.Items) != 0 {
		sort.SliceStable(jobs.Items, func(i, j int) bool {
			return jobs.Items[i].Status.StartTime.After(jobs.Items[j].Status.StartTime.Time)
		})
	}

	for i := range jobs.Items {
		job := &jobs.Items[i]
		var jobRet models.Job
		jobRet.Name = job.Name
		jobRet.Owner = job.Labels["owner"]
		jobRet.StartTime = job.Status.StartTime.Format(time.UnixDate)
		if job.Status.CompletionTime != nil {
			jobRet.CompletionTime = job.Status.CompletionTime.Format(time.UnixDate)
		}
		jobRet.Failed = job.Status.Failed
		jobRet.Completed = job.Status.Succeeded
		if jobRet.Failed == 0 && jobRet.Completed == 0 {
			jobRet.Status = "running"
		} else if jobRet.Completed != 0 {
			jobRet.Status = "completed"
		} else if jobRet.Failed != 0 {
			jobRet.Status = "failed"
		} else {
			jobRet.Status = ""
		}
		jobsList = append(jobsList, jobRet)
	}

	var filtered []models.Job
	for _, job := range jobsList {
		if params.Owner != "" && !strings.Contains(strings.ToLower(job.Owner), strings.ToLower(params.Owner)) {
			continue
		}
		if params.Name != "" && !strings.Contains(strings.ToLower(job.Name), strings.ToLower(params.Name)) {
			continue
		}
		if params.Status != "" {
			switch params.Status {
			case "completed":
				if job.Completed == 0 {
					continue
				}
			case "failed":
				if job.Failed == 0 {
					continue
				}
			case "running":
				if job.Completed != 0 || job.Failed != 0 {
					continue
				}
			}
		}
		filtered = append(filtered, job)
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

	return filtered, total, nil
}

func (j *JobServiceImpl) GetJob(username string, jobName string) (models.Job, error) {
	var jobStatus models.Job

	job, err := helpers.GetJob(j.config.Kube, jobName)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return jobStatus, fmt.Errorf("job '%s': %w", jobName, ErrSvcObjNotFound)
		}
		return jobStatus, fmt.Errorf("failed to get job '%s': %w", jobName, err)
	}

	jobStatus.Name = job.Name
	jobStatus.Owner = job.Labels["owner"]
	jobStatus.StartTime = job.Status.StartTime.Format(time.UnixDate)
	if job.Status.CompletionTime != nil {
		jobStatus.CompletionTime = job.Status.CompletionTime.Format(time.UnixDate)
	}
	jobStatus.Failed = job.Status.Failed
	jobStatus.Completed = job.Status.Succeeded

	if jobStatus.Failed == 0 && jobStatus.Completed == 0 {
		jobStatus.Status = "running"
	} else if jobStatus.Completed != 0 {
		jobStatus.Status = "completed"
	} else if jobStatus.Failed != 0 {
		jobStatus.Status = "failed"
	} else {
		jobStatus.Status = ""
	}
	labelSelector := fmt.Sprintf("job-name=%s", jobName)
	if username != "" {
		labelSelector = labelSelector + ",owner=" + username
	}

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return jobStatus, fmt.Errorf("failed to get job '%s' status: %w", jobName, err)
	}

	if len(pods.Items) == 0 {
		return jobStatus, fmt.Errorf("job '%s' - no running containers", jobName)
	}

	for i := range pods.Items {
		for c := range pods.Items[i].Status.InitContainerStatuses {
			if !pods.Items[i].Status.InitContainerStatuses[c].Ready {
				state := pods.Items[i].Status.InitContainerStatuses[c].State
				if state.Waiting != nil {
					if state.Waiting.Reason == "ImagePullBackOff" {
						jobStatus.FailReason = "failed to pull init container image from container registry."
						return jobStatus, nil
					}
				}
				if state.Terminated != nil && state.Terminated.ExitCode != 0 {
					if state.Terminated.Reason == "Error" {
						jobStatus.FailReason = "failed to clone repo: wrong repo url or incorrect credentials."
						return jobStatus, nil
					}
					return jobStatus, fmt.Errorf("init container failed with exit code %d (reason: %s)", state.Terminated.ExitCode, state.Terminated.Reason)
				}
			}
		}
		for c := range pods.Items[i].Status.ContainerStatuses {
			if !pods.Items[i].Status.ContainerStatuses[c].Ready {
				state := pods.Items[i].Status.ContainerStatuses[c].State
				if state.Waiting != nil {
					if state.Waiting.Reason == "ImagePullBackOff" {
						jobStatus.FailReason = "failed to pull application container image from container registry."
						return jobStatus, nil
					}
				}
				if state.Terminated != nil && state.Terminated.ExitCode != 0 {
					jobStatus.FailReason = fmt.Sprintf("application container failed with exit code %d (reason: %s)", state.Terminated.ExitCode, state.Terminated.Reason)
					return jobStatus, nil
				}
			}
		}
	}

	jobLog, err := j.GetLog(username, jobName)
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

func (j *JobServiceImpl) GetLog(username string, jobName string) (string, error) {
	var logs string

	// Validation if job exists
	_, err := helpers.GetJob(j.config.Kube, jobName)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return logs, fmt.Errorf("job '%s': %w", jobName, ErrSvcObjNotFound)
		}
		return logs, fmt.Errorf("failed to get logs for job '%s': %w", jobName, err)
	}

	labelSelector := "job-name=" + jobName
	if username != "" {
		labelSelector = labelSelector + ",owner=" + username
	}

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return logs, err
	}

	if len(pods.Items) == 0 {
		return logs, fmt.Errorf("job '%s' - no running containers", jobName)
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

func (j *JobServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
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
			return nil, fmt.Errorf("task '%s': failed to convert schema to json format: %w", name, err)
		}
	}

	return data, nil
}
