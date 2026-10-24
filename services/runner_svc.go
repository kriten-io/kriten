package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

type RunnerService interface {
	ListRunners([]string, models.RunnerQueryParams) ([]models.Runner, int, error)
	GetRunner(uuid.UUID) (*models.Runner, error)
	CreateRunner(models.Runner) (*models.Runner, error)
	UpdateRunner(uuid.UUID, models.Runner) (*models.Runner, error)
	DeleteRunner(uuid.UUID) error
	ListAllJobs() ([]models.Job, error)
	GetSecret(uuid.UUID) (map[string]string, error)
	UpdateSecret(uuid.UUID, map[string]string) (map[string]string, error)
	DeleteSecret(uuid.UUID) error
	// GetRunnerForExecution returns a runner with the git token and custom
	// secrets decrypted. It must only be used internally to launch jobs and
	// never returned to an API client.
	GetRunnerForExecution(uuid.UUID) (*models.Runner, error)
}

type RunnerServiceImpl struct {
	db     *gorm.DB
	config config.Config
}

func NewRunnerService(database *gorm.DB, config config.Config) RunnerService {
	return &RunnerServiceImpl{
		db:     database,
		config: config,
	}
}

// getRunnerByID fetches a runner model from the database, translating the
// not found error into the service level error.
func (r *RunnerServiceImpl) getRunnerByID(id uuid.UUID) (*models.Runner, error) {
	var runner models.Runner

	res := r.db.Where("id = ?", id).First(&runner)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("runner '%s': %w", id, ErrSvcObjNotFound)
		}
		return nil, fmt.Errorf("runner '%s': %w", id, res.Error)
	}

	return &runner, nil
}

func (r *RunnerServiceImpl) ListRunners(authList []string, params models.RunnerQueryParams) ([]models.Runner, int, error) {
	var runnersList []models.Runner

	if len(authList) == 0 {
		return runnersList, 0, nil
	}

	res := r.db.Find(&runnersList)
	if res.Error != nil {
		return nil, 0, fmt.Errorf("failed to fetch list of runners: %w", res.Error)
	}

	var authorised []models.Runner
	if slices.Contains(authList, "*") {
		authorised = runnersList
	} else {
		for _, runner := range runnersList {
			if slices.Contains(authList, runner.ID.String()) {
				authorised = append(authorised, runner)
			}
		}
	}

	var filtered []models.Runner
	for _, runner := range authorised {
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

	return filtered, total, nil
}

func (r *RunnerServiceImpl) GetRunner(id uuid.UUID) (*models.Runner, error) {
	runner, err := r.getRunnerByID(id)
	if err != nil {
		return nil, err
	}

	// never return the git token value to API clients
	if runner.TokenEncrypted != "" {
		runner.Token = "************"
	}

	secret, err := r.GetSecret(id)
	if err != nil {
		return nil, fmt.Errorf("runner '%s' secrets: %w", id, err)
	}
	runner.Secret = secret

	return runner, nil
}

func (r *RunnerServiceImpl) GetRunnerForExecution(id uuid.UUID) (*models.Runner, error) {
	runner, err := r.getRunnerByID(id)
	if err != nil {
		return nil, err
	}

	if runner.TokenEncrypted != "" {
		token, err := helpers.DecryptSecret(r.config.SecretsKey, runner.TokenEncrypted)
		if err != nil {
			return nil, fmt.Errorf("runner '%s' git repo token: %w", id, err)
		}
		runner.Token = token
	}

	var secrets []models.RunnerSecret
	if err := r.db.Where("runner_id = ?", runner.ID).Find(&secrets).Error; err != nil {
		return nil, fmt.Errorf("failed to get secrets for runner '%s': %w", id, err)
	}

	secretMap := make(map[string]string)
	for _, s := range secrets {
		value, err := helpers.DecryptSecret(r.config.SecretsKey, s.ValueEncrypted)
		if err != nil {
			return nil, fmt.Errorf("runner '%s' secret '%s': %w", id, s.Key, err)
		}
		secretMap[s.Key] = value
	}
	runner.Secret = secretMap

	return runner, nil
}

func (r *RunnerServiceImpl) CreateRunner(runner models.Runner) (*models.Runner, error) {
	err := helpers.ValidateK8sObjName(runner.Name)
	if err != nil {
		return nil, fmt.Errorf("runner '%s': %w",
			runner.Name,
			ErrSvcObjNameK8sConfMap)
	}

	if runner.Branch == "" {
		runner.Branch = "main"
	}

	newRunner := models.Runner{
		Name:   runner.Name,
		Image:  runner.Image,
		GitURL: runner.GitURL,
		Branch: runner.Branch,
	}

	if runner.Token != "" {
		tokenEncrypted, err := helpers.EncryptSecret(r.config.SecretsKey, runner.Token)
		if err != nil {
			return nil, fmt.Errorf("runner '%s' create git repo token: %w", runner.Name, err)
		}
		newRunner.TokenEncrypted = tokenEncrypted
	}

	res := r.db.Create(&newRunner)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return nil, fmt.Errorf("runner '%s': %w", runner.Name, ErrSvcObjExists)
		}
		return nil, fmt.Errorf("runner '%s': %w", runner.Name, res.Error)
	}

	if runner.Secret != nil {
		if _, err := r.UpdateSecret(newRunner.ID, runner.Secret); err != nil {
			return nil, fmt.Errorf("runner '%s' secrets update: %w", runner.Name, err)
		}
	}

	return r.GetRunner(newRunner.ID)
}

func (r *RunnerServiceImpl) UpdateRunner(id uuid.UUID, runner models.Runner) (*models.Runner, error) {
	current, err := r.getRunnerByID(id)
	if err != nil {
		return nil, err
	}

	current.Image = runner.Image
	current.GitURL = runner.GitURL
	if runner.Branch != "" {
		current.Branch = runner.Branch
	}

	switch runner.Token {
	case "************":
		// unchanged, keep the stored token
	case "":
		current.TokenEncrypted = ""
	default:
		tokenEncrypted, err := helpers.EncryptSecret(r.config.SecretsKey, runner.Token)
		if err != nil {
			return nil, fmt.Errorf("runner '%s' update git repo token: %w", current.Name, err)
		}
		current.TokenEncrypted = tokenEncrypted
	}

	res := r.db.Model(&models.Runner{}).Where("id = ?", current.ID).Updates(map[string]interface{}{
		"image":           current.Image,
		"git_url":         current.GitURL,
		"branch":          current.Branch,
		"token_encrypted": current.TokenEncrypted,
	})
	if res.Error != nil {
		return nil, fmt.Errorf("runner '%s': %w", runner.Name, res.Error)
	}

	if runner.Secret != nil {
		if _, err := r.UpdateSecret(id, runner.Secret); err != nil {
			return nil, fmt.Errorf("runner '%s' update secrets: %w", id, err)
		}
	}

	return r.GetRunner(id)
}

func (r *RunnerServiceImpl) DeleteRunner(id uuid.UUID) error {
	runner, err := r.getRunnerByID(id)
	if err != nil {
		return err
	}

	// Tasks live in the database and reference runners by UUID.
	var boundTask models.Task
	if err := r.db.Where("runner_id = ?", id).First(&boundTask).Error; err == nil {
		return fmt.Errorf("runner is bound to task: %s , delete that first: %w", boundTask.Name, ErrSvcObjInUse)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if err := r.db.Where("runner_id = ?", runner.ID).Delete(&models.RunnerSecret{}).Error; err != nil {
		return fmt.Errorf("runner '%s' secrets: %w", id, err)
	}

	res := r.db.Delete(&models.Runner{}, "id = ?", runner.ID)
	if res.Error != nil {
		return fmt.Errorf("runner '%s': %w", id, res.Error)
	}

	return nil
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

func (r *RunnerServiceImpl) GetSecret(id uuid.UUID) (map[string]string, error) {
	secretCleaned := make(map[string]string)

	runner, err := r.getRunnerByID(id)
	if err != nil {
		return nil, err
	}

	var secrets []models.RunnerSecret
	if err := r.db.Where("runner_id = ?", runner.ID).Find(&secrets).Error; err != nil {
		return nil, fmt.Errorf("failed to get secret for runner '%s': %w", id, err)
	}

	for _, s := range secrets {
		secretCleaned[s.Key] = "************"
	}

	return secretCleaned, nil
}

func (r *RunnerServiceImpl) UpdateSecret(id uuid.UUID, secret map[string]string) (map[string]string, error) {
	runner, err := r.getRunnerByID(id)
	if err != nil {
		return nil, err
	}

	var currentSecrets []models.RunnerSecret
	if err := r.db.Where("runner_id = ?", runner.ID).Find(&currentSecrets).Error; err != nil {
		return nil, fmt.Errorf("failed to get secret for runner '%s': %w", id, err)
	}

	currentMap := make(map[string]models.RunnerSecret, len(currentSecrets))
	for _, s := range currentSecrets {
		currentMap[s.Key] = s
	}

	for key, value := range secret {
		existing, ok := currentMap[key]

		switch {
		case value == "************":
			// unchanged placeholder, keep the stored value
		case value == "":
			if ok {
				if err := r.db.Delete(&existing).Error; err != nil {
					return nil, fmt.Errorf("failed to delete secret '%s' for runner '%s': %w", key, id, err)
				}
			}
		default:
			valueEncrypted, err := helpers.EncryptSecret(r.config.SecretsKey, value)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt secret '%s' for runner '%s': %w", key, id, err)
			}

			if ok {
				existing.ValueEncrypted = valueEncrypted
				if err := r.db.Save(&existing).Error; err != nil {
					return nil, fmt.Errorf("failed to update secret '%s' for runner '%s': %w", key, id, err)
				}
			} else {
				newSecret := models.RunnerSecret{
					RunnerID:       runner.ID,
					Key:            key,
					ValueEncrypted: valueEncrypted,
				}
				if err := r.db.Create(&newSecret).Error; err != nil {
					return nil, fmt.Errorf("failed to create secret '%s' for runner '%s': %w", key, id, err)
				}
			}
		}
	}

	return r.GetSecret(id)
}

func (r *RunnerServiceImpl) DeleteSecret(id uuid.UUID) error {
	runner, err := r.getRunnerByID(id)
	if err != nil {
		return fmt.Errorf("runner %s not found: %w", id, err)
	}

	if err := r.db.Where("runner_id = ?", runner.ID).Delete(&models.RunnerSecret{}).Error; err != nil {
		return fmt.Errorf("failed to delete secret '%s': %w", id, err)
	}

	return nil
}
