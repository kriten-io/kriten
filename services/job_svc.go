package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type JobService interface {
	ListJobs([]string, models.JobQueryParams) ([]models.Job, int, error)
	GetJob(uuid.UUID) (models.Job, error)
	GetLog(uuid.UUID) (string, error)
	StoreJob(models.Job) (models.Job, error)
	RefreshJob(uuid.UUID) error
	StartWorkers()
}

type JobServiceImpl struct {
	db        *gorm.DB
	config    config.Config
	queue     chan uuid.UUID
	inFlight  map[uuid.UUID]struct{}
	mu        sync.Mutex
	startOnce sync.Once
}

func NewJobService(database *gorm.DB, config config.Config) JobService {
	return &JobServiceImpl{
		db:       database,
		config:   config,
		queue:    make(chan uuid.UUID, 64),
		inFlight: make(map[uuid.UUID]struct{}),
	}
}

func findDelimitedString(str string) ([]byte, error) {
	delimiter := "^JSON"
	index := strings.Index(str, delimiter)

	if index == -1 {
		return nil, nil
	}

	start := index + len(delimiter)
	end := strings.Index(str[start:], delimiter)

	if end == -1 {
		return nil, nil
	}

	return []byte(str[start : start+end]), nil
}

func (j *JobServiceImpl) ListJobs(authList []string, params models.JobQueryParams) ([]models.Job, int, error) {
	var jobs []models.Job

	if len(authList) == 0 {
		return jobs, 0, nil
	}

	authorizeAll := false
	var taskIDs []uuid.UUID
	for _, resourceName := range authList {
		if resourceName == "*" {
			authorizeAll = true
			break
		}
		taskID, err := uuid.FromString(resourceName)
		if err != nil {
			continue
		}
		taskIDs = append(taskIDs, taskID)
	}

	if !authorizeAll && len(taskIDs) == 0 {
		return jobs, 0, nil
	}

	query := j.db.Model(&models.Job{})
	if !authorizeAll {
		query = query.Where("task_id IN ?", taskIDs)
	}

	if params.Owner != "" {
		query = query.Where("owner ILIKE ?", "%"+params.Owner+"%")
	}
	if params.Name != "" {
		name := "%" + params.Name + "%"
		query = query.Where("(name ILIKE ? OR task_name ILIKE ?)", name, name)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	ordered := query.Select("id", "name", "task_id", "task_name", "owner", "start_time", "completion_time", "failed", "completed", "status", "fail_reason", "created_at", "updated_at").Order("created_at DESC")
	if params.Limit > 0 {
		ordered = ordered.Limit(params.Limit).Offset(params.Offset)
	}

	if err := ordered.Find(&jobs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch jobs: %w", err)
	}

	return jobs, int(total), nil
}

func (j *JobServiceImpl) GetJob(id uuid.UUID) (models.Job, error) {
	var job models.Job

	res := j.db.Where("id = ?", id).First(&job)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return job, fmt.Errorf("job '%s': %w", id, ErrSvcObjNotFound)
		}
		return job, fmt.Errorf("failed to get job '%s': %w", id, res.Error)
	}

	return job, nil
}

func (j *JobServiceImpl) GetLog(id uuid.UUID) (string, error) {
	var job models.Job

	res := j.db.Select("id", "stdout").Where("id = ?", id).First(&job)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("job '%s': %w", id, ErrSvcObjNotFound)
		}
		return "", fmt.Errorf("failed to get job '%s': %w", id, res.Error)
	}

	return job.Stdout, nil
}

func (j *JobServiceImpl) StoreJob(job models.Job) (models.Job, error) {
	if job.Status == "" {
		job.Status = "running"
	}

	res := j.db.Create(&job)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return job, fmt.Errorf("job '%s': %w", job.Name, ErrSvcDBDuplicatedKey)
		}
		return job, fmt.Errorf("failed to store job '%s': %w", job.Name, res.Error)
	}

	return job, nil
}

func (j *JobServiceImpl) RefreshJob(id uuid.UUID) error {
	return j.refreshJob(id)
}

func (j *JobServiceImpl) StartWorkers() {
	j.startOnce.Do(func() {
		workerCount := j.config.Jobs.WorkerCount
		if workerCount < 1 {
			workerCount = config.JobsWorkerCountDefault
		}

		pollInterval := time.Duration(j.config.Jobs.PollInterval) * time.Second
		if pollInterval <= 0 {
			pollInterval = time.Duration(config.JobsPollIntervalDefault) * time.Second
		}

		for i := 0; i < workerCount; i++ {
			go j.worker()
		}

		go func() {
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()

			j.enqueueRunningJobs()
			for range ticker.C {
				j.enqueueRunningJobs()
			}
		}()

		log.Printf("job worker pool started with %d workers, polling every %s", workerCount, pollInterval)
	})
}

func (j *JobServiceImpl) enqueueRunningJobs() {
	var jobs []models.Job

	if err := j.db.Select("id").Where("status = ?", "running").Order("updated_at ASC").Find(&jobs).Error; err != nil {
		log.Printf("failed to list running jobs: %v", err)
		return
	}

	for _, job := range jobs {
		j.mu.Lock()
		if _, exists := j.inFlight[job.ID]; exists {
			j.mu.Unlock()
			continue
		}
		j.inFlight[job.ID] = struct{}{}
		j.mu.Unlock()

		select {
		case j.queue <- job.ID:
		default:
			j.release(job.ID)
		}
	}
}

func (j *JobServiceImpl) worker() {
	for id := range j.queue {
		func() {
			defer j.release(id)

			if err := j.refreshJob(id); err != nil {
				log.Printf("failed to refresh job '%s': %v", id, err)
			}
		}()
	}
}

func (j *JobServiceImpl) release(id uuid.UUID) {
	j.mu.Lock()
	delete(j.inFlight, id)
	j.mu.Unlock()
}

func (j *JobServiceImpl) refreshJob(id uuid.UUID) error {
	var job models.Job

	if err := j.db.Where("id = ?", id).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("job '%s': %w", id, ErrSvcObjNotFound)
		}
		return fmt.Errorf("failed to get job '%s': %w", id, err)
	}

	if job.Status != "running" {
		return nil
	}

	latest, err := j.getJobFromK8s(job.Name)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return j.db.Model(&models.Job{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
				"status":      "failed",
				"fail_reason": "job not found in cluster",
			}).Error
		}
		return fmt.Errorf("failed to refresh job '%s': %w", job.Name, err)
	}

	updates := map[string]interface{}{
		"failed":    latest.Failed,
		"completed": latest.Completed,
		"status":    latest.Status,
	}
	if latest.StartTime != "" {
		updates["start_time"] = latest.StartTime
	}
	if latest.CompletionTime != "" {
		updates["completion_time"] = latest.CompletionTime
	}
	if latest.FailReason != "" {
		updates["fail_reason"] = latest.FailReason
	}
	if latest.Stdout != "" {
		updates["stdout"] = latest.Stdout
	}
	if latest.JsonData != nil {
		if b, err := json.Marshal(latest.JsonData); err == nil {
			updates["json_data"] = string(b)
		} else {
			updates["json_data"] = latest.JsonData
		}
	}

	if err := j.db.Model(&models.Job{}).Where("id = ?", job.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update job '%s': %w", job.Name, err)
	}

	return nil
}

func (j *JobServiceImpl) getJobFromK8s(jobName string) (models.Job, error) {
	var jobStatus models.Job

	job, err := helpers.GetJob(j.config.Kube, jobName)
	if err != nil {
		return jobStatus, err
	}

	jobStatus.Name = job.Name
	jobStatus.Owner = job.Labels["owner"]
	if !job.Status.StartTime.IsZero() {
		jobStatus.StartTime = job.Status.StartTime.Format(time.UnixDate)
	}
	if job.Status.CompletionTime != nil && !job.Status.CompletionTime.IsZero() {
		jobStatus.CompletionTime = job.Status.CompletionTime.Format(time.UnixDate)
	}
	jobStatus.Failed = job.Status.Failed
	jobStatus.Completed = job.Status.Succeeded
	jobStatus.Status = jobStatusFromK8s(job.Status.Failed, job.Status.Succeeded)

	labelSelector := fmt.Sprintf("job-name=%s", jobName)

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return jobStatus, fmt.Errorf("failed to get job '%s' status: %w", jobName, err)
	}

	if len(pods.Items) == 0 {
		return jobStatus, nil
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

	jobLog, err := j.getK8sLog(jobName)
	if err != nil {
		jobStatus.Stdout = fmt.Sprintf("failed to read logs from containers: %v", err)
	} else {
		jobStatus.Stdout = jobLog
	}

	if jobStatus.Stdout != "" && jobStatus.Status != "running" {
		jobStatus.JsonData = parseJobJSON(jobStatus.Stdout)
	}

	return jobStatus, nil
}

func (j *JobServiceImpl) getK8sLog(jobName string) (string, error) {
	var logs string

	labelSelector := "job-name=" + jobName

	pods, err := helpers.ListPods(j.config.Kube, labelSelector)
	if err != nil {
		return logs, err
	}

	if len(pods.Items) == 0 {
		return logs, fmt.Errorf("job '%s' - no running containers", jobName)
	}

	for _, pod := range pods.Items {
		for c := range pod.Spec.InitContainers {
			jobLog, err := helpers.GetLogs(j.config.Kube, pod.Name, pod.Spec.InitContainers[c].Name)
			if err != nil {
				logs += fmt.Sprintf("error reading logs from init container: %v", err)
			} else if jobLog != "" {
				logs += "\n\n## init container logs\n"
				logs += jobLog
			}
		}
		for c := range pod.Spec.Containers {
			jobLog, err := helpers.GetLogs(j.config.Kube, pod.Name, pod.Spec.Containers[c].Name)
			if err != nil {
				logs += fmt.Sprintf("error reading logs from application container: %v", err)
			} else if jobLog != "" {
				logs += "\n\n##application container logs \n"
				logs += jobLog
			}
		}
	}

	return logs, nil
}

func jobStatusFromK8s(failed int32, completed int32) string {
	if failed == 0 && completed == 0 {
		return "running"
	}
	if completed != 0 {
		return "completed"
	}
	return "failed"
}

func parseJobJSON(stdout string) map[string]interface{} {
	jsonByte, _ := findDelimitedString(stdout)
	if jsonByte == nil {
		return nil
	}

	replacer := strings.NewReplacer("\n", "", "\\", "")
	jsonString := replacer.Replace(string(jsonByte))

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
		return map[string]interface{}{"error": "failed to parse JSON"}
	}

	return data
}
