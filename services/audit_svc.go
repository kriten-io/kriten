package services

import (
	"log"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	"gorm.io/gorm"
)

type AuditService interface {
	ListAuditLogs(models.AuditQueryParams) ([]models.AuditLog, int, error)
	GetAuditLog(string) (models.AuditLog, error)
	CreateAudit(models.AuditLog)
	NewAuditLog(models.Actor, string, string, string) models.AuditLog
}

type AuditServiceImpl struct {
	db     *gorm.DB
	config config.Config
}

func NewAuditService(database *gorm.DB, config config.Config) AuditService {
	return &AuditServiceImpl{
		db:     database,
		config: config,
	}
}

func (a *AuditServiceImpl) ListAuditLogs(params models.AuditQueryParams) ([]models.AuditLog, int, error) {
	var logs []models.AuditLog
	res := a.db.Order("created_at desc").Find(&logs)
	if res.Error != nil {
		return logs, 0, res.Error
	}

	total := len(logs)

	if params.Limit > 0 {
		start := params.Offset
		if start > total {
			start = total
		}
		end := start + params.Limit
		if end > total {
			end = total
		}
		logs = logs[start:end]
	}

	return logs, total, nil
}

func (a *AuditServiceImpl) GetAuditLog(id string) (models.AuditLog, error) {
	var log models.AuditLog
	res := a.db.Where("auditlog_id = ?", id).Find(&log)
	if res.Error != nil {
		return models.AuditLog{}, res.Error
	}

	if res.RowsAffected == 0 {
		return models.AuditLog{}, gorm.ErrRecordNotFound
	}

	return log, nil
}

func (a *AuditServiceImpl) CreateAudit(auditlog models.AuditLog) {
	res := a.db.Create(&auditlog)
	if res.Error != nil {
		log.Println("Error during Audit creation: " + res.Error.Error())
	}
}

func (a *AuditServiceImpl) NewAuditLog(
	actor models.Actor,
	eventType string,
	category string,
	target string,
) models.AuditLog {
	return models.AuditLog{
		UserID:        actor.UserID,
		UserName:      actor.Username,
		Provider:      actor.Provider,
		EventType:     eventType,
		EventCategory: category,
		EventTarget:   target,
		Status:        "error", // status will be updated later if successful
	}
}
