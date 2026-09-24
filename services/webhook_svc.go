package services

import (
	"fmt"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	"gorm.io/gorm"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/exp/slices"
)

type WebhookService interface {
	ListWebhooks(uuid.UUID) ([]models.Webhook, error)
	ListTaskWebhooks(string) ([]models.Webhook, error)
	ListAllWebhooks([]string) ([]models.Webhook, error)
	GetWebhook(string) (models.Webhook, error)
	CreateWebhook(models.Actor, models.Webhook) (models.Webhook, error)
	DeleteWebhook(models.Actor, string) error
}

const webhooksCategory = "webHooks"

type WebhookServiceImpl struct {
	db     *gorm.DB
	config config.Config
	audit  AuditService
}

func NewWebhookService(database *gorm.DB, config config.Config, als AuditService) WebhookService {
	return &WebhookServiceImpl{
		db:     database,
		config: config,
		audit:  als,
	}
}

func (w *WebhookServiceImpl) ListWebhooks(userid uuid.UUID) ([]models.Webhook, error) {
	var webHooks []models.Webhook

	res := w.db.Select("id", "owner", "secret", "description", "task", "created_at", "updated_at").
		Where("owner = ?", userid).
		Find(&webHooks)

	if res.Error != nil {
		return webHooks, fmt.Errorf("failed to get user webhooks: %w", res.Error)
	}

	return webHooks, nil
}

func (w *WebhookServiceImpl) ListTaskWebhooks(taskName string) ([]models.Webhook, error) {
	var webHooks []models.Webhook

	res := w.db.Select("id", "owner", "secret", "description", "task", "created_at", "updated_at").
		Where("task = ?", taskName).
		Find(&webHooks)

	if res.Error != nil {
		return webHooks, fmt.Errorf("failed to get task webhooks: %w", res.Error)
	}

	return webHooks, nil
}

func (w *WebhookServiceImpl) ListAllWebhooks(authList []string) ([]models.Webhook, error) {
	var webHooks []models.Webhook
	var res *gorm.DB

	if len(authList) == 0 {
		return webHooks, nil
	}

	if slices.Contains(authList, "*") {
		res = w.db.Find(&webHooks)
	} else {
		res = w.db.Find(&webHooks, authList)
	}
	if res.Error != nil {
		return webHooks, fmt.Errorf("failed to get all webhooks: %w", res.Error)
	}

	return webHooks, nil
}

func (w *WebhookServiceImpl) GetWebhook(id string) (models.Webhook, error) {
	var webHook models.Webhook

	res := w.db.Select("id", "owner", "secret", "description", "task", "created_at", "updated_at").
		Where("id = ?", id).
		Find(&webHook)

	if res.Error != nil {
		return models.Webhook{}, fmt.Errorf("failed to find webhook '%s': %w", id, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.Webhook{}, fmt.Errorf("webhook '%s': %w", id, ErrSvcObjNotFound)
	}

	return webHook, nil
}

func (w *WebhookServiceImpl) CreateWebhook(actor models.Actor, webHook models.Webhook) (models.Webhook, error) {
	audit := w.audit.NewAuditLog(actor, "create", webhooksCategory, webHook.Task)
	webHook.Owner = actor.UserID

	res := w.db.Create(&webHook)
	if res.Error != nil {
		w.audit.CreateAudit(audit)
		return models.Webhook{}, fmt.Errorf("failed to create webhook: %w", res.Error)
	}
	audit.Status = "success"
	w.audit.CreateAudit(audit)
	return webHook, nil
}

func (w *WebhookServiceImpl) DeleteWebhook(actor models.Actor, id string) error {
	audit := w.audit.NewAuditLog(actor, "delete", webhooksCategory, id)

	webHook, err := w.GetWebhook(id)
	if err != nil {
		w.audit.CreateAudit(audit)
		return fmt.Errorf("failed to get webhook '%s': %w", id, err)
	}
	res := w.db.Unscoped().Delete(&webHook)
	if res.Error != nil {
		w.audit.CreateAudit(audit)
		return fmt.Errorf("failed to delete webhook '%s': %w", id, res.Error)
	}
	audit.Status = "success"
	w.audit.CreateAudit(audit)
	return nil
}
