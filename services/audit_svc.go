package services

import (
	"fmt"
	"kriten/config"
	"kriten/models"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

type AuditService interface {
	ListAudits([]string) ([]models.AuditLog, error)
	GetAudit(string) (models.AuditLog, error)
	CreateAudit(models.AuditLog) (models.AuditLog, error)
	InitialiseAuditLog(*gin.Context, string, string) models.AuditLog
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

func (a *AuditServiceImpl) ListAudits(authList []string) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	var res *gorm.DB

	if len(authList) == 0 {
		return logs, nil
	}

	if slices.Contains(authList, "*") {
		res = a.db.Find(&logs)
	} else {
		res = a.db.Find(&logs, authList)
	}
	if res.Error != nil {
		return logs, res.Error
	}

	return logs, nil
}

func (a *AuditServiceImpl) GetAudit(id string) (models.AuditLog, error) {
	var log models.AuditLog
	res := a.db.Where("auditlog_id = ?", id).Find(&log)
	if res.Error != nil {
		return models.AuditLog{}, res.Error
	}

	if log.UserName == "" {
		return models.AuditLog{}, fmt.Errorf("Audit log %s not found, please check uuid", id)
	}

	return log, nil
}

func (a *AuditServiceImpl) CreateAudit(log models.AuditLog) (models.AuditLog, error) {
	res := a.db.Create(&log)

	return log, res.Error
}

func (a *AuditServiceImpl) InitialiseAuditLog(ctx *gin.Context, eventType string, category string) models.AuditLog {
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
		EventTarget:   "*",
		Status:        "error", // status will be updated later if successful
	}
}
