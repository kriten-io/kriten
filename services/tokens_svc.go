package services

import (
	"crypto/rand"
	"encoding/hex"
	"kriten/config"
	"kriten/models"

	uuid "github.com/satori/go.uuid"

	"gorm.io/gorm"
)

type ApiTokenService interface {
	ListApiTokens(uuid.UUID) ([]models.ApiToken, error)
	GetApiToken(string) (models.ApiToken, error)
	IsTokenValid(string) (bool, error)
	CreateApiToken(models.ApiToken) (models.ApiToken, error)
	UpdateApiToken(models.ApiToken) (models.ApiToken, error)
	DeleteApiToken(string) error
}

type ApiTokenServiceImpl struct {
	db     *gorm.DB
	config config.Config
}

func NewApiTokenService(database *gorm.DB, config config.Config) ApiTokenService {
	return &ApiTokenServiceImpl{
		db:     database,
		config: config,
	}
}

func (u *ApiTokenServiceImpl) ListApiTokens(userid uuid.UUID) ([]models.ApiToken, error) {
	var apiTokens []models.ApiToken
	var res *gorm.DB

	res = u.db.Where("owner = ?", userid).Find(&apiTokens)

	if res.Error != nil {
		return apiTokens, res.Error
	}

	return apiTokens, nil
}

func (u *ApiTokenServiceImpl) GetApiToken(id string) (models.ApiToken, error) {
	var apiToken models.ApiToken
	res := u.db.Where("id = ?", id).Find(&apiToken)
	if res.Error != nil {
		return models.ApiToken{}, res.Error
	}

	return apiToken, nil
}

func (u *ApiTokenServiceImpl) IsTokenValid(key string) (bool, error) {
	var apiToken models.ApiToken
	res := u.db.Where("key = ?", key).Find(&apiToken)
	if res.Error != nil {
		return false, res.Error
	}

	return apiToken.Enabled, nil
}

func (u *ApiTokenServiceImpl) CreateApiToken(apiToken models.ApiToken) (models.ApiToken, error) {
	apiToken.Key, _ = randomHex(40)

	res := u.db.Create(&apiToken)

	return apiToken, res.Error
}

func (u *ApiTokenServiceImpl) UpdateApiToken(apiToken models.ApiToken) (models.ApiToken, error) {
	newApiToken, err := u.GetApiToken(apiToken.ID.String())
	if err != nil {
		return models.ApiToken{}, err
	}
	return newApiToken, nil
}

func (u *ApiTokenServiceImpl) DeleteApiToken(id string) error {
	apiToken, err := u.GetApiToken(id)
	if err != nil {
		return err
	}
	return u.db.Unscoped().Delete(&apiToken).Error
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
