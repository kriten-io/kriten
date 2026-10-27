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
	"github.com/kriten-io/kriten/models"

	"github.com/robfig/cron/v3"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const schedulerBatchSize = 50

// cronParser accepts the standard 5 field cron syntax plus descriptors such as
// @daily. Schedules are evaluated in UTC.
var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

type SchedulerService interface {
	ListSchedulers(models.AuditMetadata, models.SchedulerQueryParams) ([]models.Scheduler, int, error)
	GetScheduler(models.AuditMetadata, string) (models.Scheduler, error)
	CreateScheduler(models.AuditMetadata, models.Scheduler) (models.Scheduler, error)
	UpdateScheduler(models.AuditMetadata, string, models.Scheduler) (models.Scheduler, error)
	DeleteScheduler(models.AuditMetadata, string) error
	EnableScheduler(models.AuditMetadata, string) (models.Scheduler, error)
	DisableScheduler(models.AuditMetadata, string) (models.Scheduler, error)
	StartScheduler()
}

type SchedulerServiceImpl struct {
	db           *gorm.DB
	config       config.Config
	auditService AuditService
	authService  AuthService
	taskService  TaskService
	startOnce    sync.Once
}

func NewSchedulerService(
	database *gorm.DB,
	config config.Config,
	als AuditService,
	as AuthService,
	ts TaskService,
) SchedulerService {
	return &SchedulerServiceImpl{
		db:           database,
		config:       config,
		auditService: als,
		authService:  as,
		taskService:  ts,
	}
}

// canAccess allows the schedule owner, or any user authorised on the
// "schedulers" resource for the given access type.
func (s *SchedulerServiceImpl) canAccess(meta models.AuditMetadata, owner uuid.UUID, schedulerID string, access string) (bool, error) {
	if owner == meta.UserID {
		return true, nil
	}

	return s.authService.IsAutorised(&models.Authorization{
		UserID:       meta.UserID,
		Provider:     meta.Provider,
		Resource:     "schedulers",
		ResourceName: schedulerID,
		Access:       access,
	})
}

func (s *SchedulerServiceImpl) loadScheduler(id string) (models.Scheduler, error) {
	var sc models.Scheduler

	res := s.db.Where("id = ?", id).Find(&sc)
	if res.Error != nil {
		return models.Scheduler{}, fmt.Errorf("failed to find scheduler '%s': %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return models.Scheduler{}, fmt.Errorf("scheduler '%s': %w", id, ErrSvcObjNotFound)
	}

	return sc, nil
}

// authorise loads a schedule and checks the caller may perform access on it.
func (s *SchedulerServiceImpl) authorise(meta models.AuditMetadata, id string, access string) (models.Scheduler, error) {
	sc, err := s.loadScheduler(id)
	if err != nil {
		return models.Scheduler{}, err
	}

	ok, err := s.canAccess(meta, sc.Owner, sc.ID.String(), access)
	if err != nil {
		return models.Scheduler{}, fmt.Errorf("scheduler '%s' authorisation: %w", id, err)
	}
	if !ok {
		return models.Scheduler{}, fmt.Errorf("scheduler '%s': %w", id, ErrSvcForbidden)
	}

	return sc, nil
}

// validateDefinition checks the cron expression, that the caller may execute
// the referenced task, and that the extra vars match the task's current
// schema. It sets TaskName and normalises ExtraVars on sc.
func (s *SchedulerServiceImpl) validateDefinition(meta models.AuditMetadata, sc *models.Scheduler) (cron.Schedule, error) {
	schedule, err := cronParser.Parse(sc.CronExpr)
	if err != nil {
		return nil, fmt.Errorf("cron expression '%s': %w, %s", sc.CronExpr, ErrSvcSchedulerCron, err)
	}

	task, err := s.taskService.GetTask(sc.TaskID)
	if err != nil {
		return nil, err
	}

	allowed, err := s.authService.IsAutorised(&models.Authorization{
		UserID:       meta.UserID,
		Provider:     meta.Provider,
		Resource:     "tasks",
		ResourceName: task.ID.String(),
		Access:       "execute",
	})
	if err != nil {
		return nil, fmt.Errorf("task '%s' authorisation: %w", task.Name, err)
	}
	if !allowed {
		return nil, fmt.Errorf("task '%s': %w", task.Name, ErrSvcForbidden)
	}

	if sc.ExtraVars == nil {
		sc.ExtraVars = map[string]any{}
	}
	extraVars, err := json.Marshal(sc.ExtraVars)
	if err != nil {
		return nil, fmt.Errorf("scheduler '%s' extra vars: %w", sc.Name, err)
	}
	if err := ValidateExtraVars(task.Schema, string(extraVars)); err != nil {
		errString := strings.ReplaceAll(err.Error(), "\"", "'")
		return nil, fmt.Errorf("scheduler '%s' extra vars: %w, %s", sc.Name, ErrSvcTaskSchemaValidation, errString)
	}

	sc.TaskName = task.Name
	return schedule, nil
}

func (s *SchedulerServiceImpl) ListSchedulers(meta models.AuditMetadata, params models.SchedulerQueryParams) ([]models.Scheduler, int, error) {
	// Users with read access to all schedulers see everything, others see their own.
	all, err := s.authService.IsAutorised(&models.Authorization{
		UserID:       meta.UserID,
		Provider:     meta.Provider,
		Resource:     "schedulers",
		ResourceName: "*",
		Access:       "read",
	})
	if err != nil {
		return nil, 0, fmt.Errorf("schedulers authorisation: %w", err)
	}

	query := func() *gorm.DB {
		q := s.db.Model(&models.Scheduler{})
		if !all {
			q = q.Where("owner = ?", meta.UserID)
		}
		if params.Name != "" {
			q = q.Where("name ILIKE ?", "%"+params.Name+"%")
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count schedulers: %w", err)
	}

	list := []models.Scheduler{}
	q := query().Order("name")
	if params.Limit > 0 {
		q = q.Limit(params.Limit).Offset(params.Offset)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list schedulers: %w", err)
	}

	return list, int(total), nil
}

func (s *SchedulerServiceImpl) GetScheduler(meta models.AuditMetadata, id string) (models.Scheduler, error) {
	return s.authorise(meta, id, "read")
}

func (s *SchedulerServiceImpl) CreateScheduler(meta models.AuditMetadata, sc models.Scheduler) (models.Scheduler, error) {
	audit := s.auditService.InitialiseAuditLog(meta, "create", "schedulers", sc.Name)
	defer func() { s.auditService.CreateAudit(audit) }()

	schedule, err := s.validateDefinition(meta, &sc)
	if err != nil {
		return models.Scheduler{}, err
	}

	next := schedule.Next(time.Now().UTC())
	sc.Owner = meta.UserID
	sc.Enabled = true
	sc.NextRunAt = &next
	sc.LastRunAt = nil
	sc.LastError = ""

	if res := s.db.Create(&sc); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return models.Scheduler{}, fmt.Errorf("scheduler '%s': %w", sc.Name, ErrSvcObjExists)
		}
		return models.Scheduler{}, fmt.Errorf("failed to create scheduler '%s': %w", sc.Name, res.Error)
	}

	audit.EventTarget = sc.ID.String()
	audit.Status = "success"
	return sc, nil
}

func (s *SchedulerServiceImpl) UpdateScheduler(meta models.AuditMetadata, id string, in models.Scheduler) (models.Scheduler, error) {
	audit := s.auditService.InitialiseAuditLog(meta, "update", "schedulers", id)
	defer func() { s.auditService.CreateAudit(audit) }()

	existing, err := s.authorise(meta, id, "write")
	if err != nil {
		return models.Scheduler{}, err
	}

	// Name is immutable, as for tasks.
	in.Name = existing.Name
	schedule, err := s.validateDefinition(meta, &in)
	if err != nil {
		return models.Scheduler{}, err
	}

	existing.Description = in.Description
	existing.TaskID = in.TaskID
	existing.TaskName = in.TaskName
	existing.CronExpr = in.CronExpr
	existing.ExtraVars = in.ExtraVars
	existing.LastError = ""
	existing.Enabled = in.Enabled
	if existing.Enabled {
		next := schedule.Next(time.Now().UTC())
		existing.NextRunAt = &next
	}

	res := s.db.Model(&existing).
		Select("description", "task_id", "task_name", "cron_expr", "extra_vars", "enabled", "last_error", "next_run_at", "updated_at").
		Updates(existing)
	if res.Error != nil {
		return models.Scheduler{}, fmt.Errorf("failed to update scheduler '%s': %w", existing.Name, res.Error)
	}

	audit.EventTarget = existing.ID.String()
	audit.Status = "success"
	return existing, nil
}

func (s *SchedulerServiceImpl) DeleteScheduler(meta models.AuditMetadata, id string) error {
	audit := s.auditService.InitialiseAuditLog(meta, "delete", "schedulers", id)
	defer func() { s.auditService.CreateAudit(audit) }()

	sc, err := s.authorise(meta, id, "write")
	if err != nil {
		return err
	}

	if res := s.db.Delete(&models.Scheduler{}, "id = ?", sc.ID); res.Error != nil {
		return fmt.Errorf("failed to delete scheduler '%s': %w", id, res.Error)
	}

	audit.Status = "success"
	return nil
}

func (s *SchedulerServiceImpl) EnableScheduler(meta models.AuditMetadata, id string) (models.Scheduler, error) {
	audit := s.auditService.InitialiseAuditLog(meta, "enable", "schedulers", id)
	defer func() { s.auditService.CreateAudit(audit) }()

	sc, err := s.authorise(meta, id, "write")
	if err != nil {
		return models.Scheduler{}, err
	}

	schedule, err := s.validateDefinition(meta, &sc)
	if err != nil {
		return models.Scheduler{}, err
	}

	next := schedule.Next(time.Now().UTC())
	sc.Enabled = true
	sc.NextRunAt = &next
	sc.LastError = ""

	res := s.db.Model(&sc).
		Select("enabled", "next_run_at", "last_error", "task_name", "extra_vars", "updated_at").
		Updates(sc)
	if res.Error != nil {
		return models.Scheduler{}, fmt.Errorf("failed to enable scheduler '%s': %w", id, res.Error)
	}

	audit.Status = "success"
	return sc, nil
}

func (s *SchedulerServiceImpl) DisableScheduler(meta models.AuditMetadata, id string) (models.Scheduler, error) {
	audit := s.auditService.InitialiseAuditLog(meta, "disable", "schedulers", id)
	defer func() { s.auditService.CreateAudit(audit) }()

	sc, err := s.authorise(meta, id, "write")
	if err != nil {
		return models.Scheduler{}, err
	}

	sc.Enabled = false
	sc.NextRunAt = nil

	res := s.db.Model(&sc).
		Select("enabled", "next_run_at", "updated_at").
		Updates(sc)
	if res.Error != nil {
		return models.Scheduler{}, fmt.Errorf("failed to disable scheduler '%s': %w", id, res.Error)
	}

	audit.Status = "success"
	return sc, nil
}

// StartScheduler starts the polling loop that fires due schedules.
func (s *SchedulerServiceImpl) StartScheduler() {
	s.startOnce.Do(func() {
		pollInterval := time.Duration(s.config.Scheduler.PollInterval) * time.Second
		if pollInterval <= 0 {
			pollInterval = time.Duration(config.SchedulerPollIntervalDefault) * time.Second
		}

		go func() {
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()

			s.runDueSchedules()
			for range ticker.C {
				s.runDueSchedules()
			}
		}()

		log.Printf("scheduler started, polling every %s", pollInterval)
	})
}

func (s *SchedulerServiceImpl) runDueSchedules() {
	now := time.Now().UTC()

	due, err := s.claimDue(now)
	if err != nil {
		log.Printf("scheduler: failed to claim due schedules: %v", err)
		return
	}

	for _, sc := range due {
		s.fire(sc, now)
	}
}

// claimDue locks due schedules with SKIP LOCKED, advances their next run time
// and commits. Other replicas skip the claimed rows, so each run fires once.
func (s *SchedulerServiceImpl) claimDue(now time.Time) ([]models.Scheduler, error) {
	var claimed []models.Scheduler

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var due []models.Scheduler
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("enabled = ? AND next_run_at <= ?", true, now).
			Order("next_run_at").
			Limit(schedulerBatchSize).
			Find(&due).Error; err != nil {
			return err
		}

		for _, sc := range due {
			schedule, err := cronParser.Parse(sc.CronExpr)
			if err != nil {
				// Cron is validated on store, so this only happens on manual DB edits.
				if err := tx.Model(&models.Scheduler{}).Where("id = ?", sc.ID).Updates(map[string]any{
					"enabled":     false,
					"next_run_at": nil,
					"last_error":  fmt.Sprintf("invalid cron expression: %v", err),
				}).Error; err != nil {
					return err
				}
				continue
			}

			if err := tx.Model(&models.Scheduler{}).Where("id = ?", sc.ID).
				Update("next_run_at", schedule.Next(now)).Error; err != nil {
				return err
			}
			claimed = append(claimed, sc)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return claimed, nil
}

// fire runs one claimed schedule as its owner. Ownership and schema problems
// disable the schedule; other errors are recorded and the schedule stays enabled.
func (s *SchedulerServiceImpl) fire(sc models.Scheduler, now time.Time) {
	var owner models.User
	if err := s.db.Where("id = ?", sc.Owner).First(&owner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.disable(sc, models.AuditMetadata{UserID: sc.Owner}, "owner no longer exists")
			return
		}
		s.recordError(sc, fmt.Errorf("failed to load owner: %w", err))
		return
	}

	meta := models.AuditMetadata{UserID: owner.ID, UserName: owner.Username, Provider: owner.Provider}
	audit := s.auditService.InitialiseAuditLog(meta, "run", "schedulers", sc.ID.String())
	defer func() { s.auditService.CreateAudit(audit) }()

	task, err := s.taskService.GetTask(sc.TaskID)
	if err != nil {
		s.recordError(sc, fmt.Errorf("failed to load task: %w", err))
		return
	}

	allowed, err := s.authService.IsAutorised(&models.Authorization{
		UserID:       owner.ID,
		Provider:     owner.Provider,
		Resource:     "tasks",
		ResourceName: task.ID.String(),
		Access:       "execute",
	})
	if err != nil {
		s.recordError(sc, fmt.Errorf("owner authorisation: %w", err))
		return
	}
	if !allowed {
		s.disable(sc, meta, "owner no longer has execute permission on task")
		return
	}

	if sc.ExtraVars == nil {
		sc.ExtraVars = map[string]any{}
	}
	extraVars, err := json.Marshal(sc.ExtraVars)
	if err != nil {
		s.recordError(sc, fmt.Errorf("extra vars: %w", err))
		return
	}
	if err := ValidateExtraVars(task.Schema, string(extraVars)); err != nil {
		s.disable(sc, meta, fmt.Sprintf("extra vars do not match task schema: %v", err))
		return
	}

	if _, err := s.taskService.ExecuteTask(owner.Username, task.ID, string(extraVars)); err != nil {
		s.recordError(sc, err)
		return
	}

	if err := s.db.Model(&models.Scheduler{}).Where("id = ?", sc.ID).
		Updates(map[string]any{"last_run_at": now, "last_error": ""}).Error; err != nil {
		log.Printf("scheduler '%s': failed to record last run: %v", sc.ID, err)
	}
	audit.Status = "success"
}

func (s *SchedulerServiceImpl) recordError(sc models.Scheduler, runErr error) {
	log.Printf("scheduler '%s' run failed: %v", sc.ID, runErr)
	if err := s.db.Model(&models.Scheduler{}).Where("id = ?", sc.ID).
		Update("last_error", runErr.Error()).Error; err != nil {
		log.Printf("scheduler '%s': failed to record error: %v", sc.ID, err)
	}
}

func (s *SchedulerServiceImpl) disable(sc models.Scheduler, meta models.AuditMetadata, reason string) {
	log.Printf("scheduler '%s' disabled: %s", sc.ID, reason)
	if err := s.db.Model(&models.Scheduler{}).Where("id = ?", sc.ID).Updates(map[string]any{
		"enabled":     false,
		"next_run_at": nil,
		"last_error":  reason,
	}).Error; err != nil {
		log.Printf("scheduler '%s': failed to disable: %v", sc.ID, err)
		return
	}

	audit := s.auditService.InitialiseAuditLog(meta, "auto_disable", "schedulers", sc.ID.String())
	audit.Status = "success"
	s.auditService.CreateAudit(audit)
}
