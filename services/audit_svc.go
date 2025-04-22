package services

import (
	"fmt"
	"log"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type AuditService interface {
	ListAuditLogs(int) ([]models.AuditLog, error)
	GetAuditLog(string) (models.AuditLog, error)
	CreateAudit(models.AuditLog)
	InitialiseAuditLog(*gin.Context, string, string, string) models.AuditLog
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

func (a *AuditServiceImpl) ListAuditLogs(num int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	res := a.db.Order("created_at desc").Limit(num).Find(&logs)
	if res.Error != nil {
		return logs, res.Error
	}

	return logs, nil
}

func (a *AuditServiceImpl) GetAuditLog(id string) (models.AuditLog, error) {
	var log models.AuditLog
	res := a.db.Where("auditlog_id = ?", id).Find(&log)
	if res.Error != nil {
		return models.AuditLog{}, res.Error
	}

	if log.UserName == "" {
		return models.AuditLog{}, fmt.Errorf("audit log %s not found, please check uuid", id)
	}

	return log, nil
}

func (a *AuditServiceImpl) CreateAudit(auditlog models.AuditLog) {
	res := a.db.Create(&auditlog)
	if res.Error != nil {
		log.Println("Error during Audit creation: " + res.Error.Error())
	}
}

func (a *AuditServiceImpl) InitialiseAuditLog(
	ctx *gin.Context,
	eventType string,
	category string,
	target string,
) models.AuditLog {
	var userID uuid.UUID
	var username, provider string
	uid, _ := ctx.Get("userID")
	if uid != nil {
		userID = uid.(uuid.UUID)
		uname, _ := ctx.Get("username")
		prov, _ := ctx.Get("provider")

		username = uname.(string)
		provider = prov.(string)
	}

	return models.AuditLog{
		UserID:        userID,
		UserName:      username,
		Provider:      provider,
		EventType:     eventType,
		EventCategory: category,
		EventTarget:   target,
		Status:        "error", // status will be updated later if successful
	}
}
