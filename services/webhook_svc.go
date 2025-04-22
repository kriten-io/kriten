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
	ListAllWebhooks([]string) ([]models.Webhook, error)
	GetWebhook(string) (models.Webhook, error)
	CreateWebhook(models.Webhook) (models.Webhook, error)
	DeleteWebhook(string) error
}

type WebhookServiceImpl struct {
	db     *gorm.DB
	config config.Config
}

func NewWebhookService(database *gorm.DB, config config.Config) WebhookService {
	return &WebhookServiceImpl{
		db:     database,
		config: config,
	}
}

func (w *WebhookServiceImpl) ListWebhooks(userid uuid.UUID) ([]models.Webhook, error) {
	var webHooks []models.Webhook

	res := w.db.Select("id", "owner", "task", "created_at", "updated_at", "description").
		Where("owner = ?", userid).
		Find(&webHooks)

	if res.Error != nil {
		return webHooks, res.Error
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
		return webHooks, res.Error
	}

	return webHooks, nil
}

func (w *WebhookServiceImpl) GetWebhook(id string) (models.Webhook, error) {
	var webHook models.Webhook

	res := w.db.Select("id", "owner", "task", "created_at", "updated_at", "description").
		Where("id = ?", id).
		Find(&webHook)

	if res.Error != nil {
		return models.Webhook{}, res.Error
	}

	if res.RowsAffected == 0 {
		return models.Webhook{}, fmt.Errorf("webhook %s not found, please check uuid", id)
	}

	return webHook, nil
}

func (w *WebhookServiceImpl) CreateWebhook(webHook models.Webhook) (models.Webhook, error) {
	res := w.db.Create(&webHook)

	return webHook, res.Error
}

func (w *WebhookServiceImpl) DeleteWebhook(id string) error {
	webHook, err := w.GetWebhook(id)
	if err != nil {
		return err
	}
	return w.db.Unscoped().Delete(&webHook).Error
}
