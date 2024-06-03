package services

import (
	"crypto/rand"
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"math/big"
	"slices"
	"time"

	uuid "github.com/satori/go.uuid"

	"gorm.io/gorm"
)

type ApiTokenService interface {
	ListApiTokens(uuid.UUID) ([]models.ApiToken, error)
	ListAllApiTokens([]string) ([]models.ApiToken, error)
	GetApiToken(string) (models.ApiToken, error)
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

	res := u.db.Where("owner = ?", userid).Find(&apiTokens)

	if res.Error != nil {
		return apiTokens, res.Error
	}

	return apiTokens, nil
}

func (u *ApiTokenServiceImpl) ListAllApiTokens(authList []string) ([]models.ApiToken, error) {
	var apiTokens []models.ApiToken
	var res *gorm.DB

	if len(authList) == 0 {
		return apiTokens, nil
	}

	if slices.Contains(authList, "*") {
		res = u.db.Find(&apiTokens)
	} else {
		res = u.db.Find(&apiTokens, authList)
	}
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

	if res.RowsAffected == 0 {
		return models.ApiToken{}, fmt.Errorf("token %s not found, please check uuid", id)
	}

	return apiToken, nil
}

func (u *ApiTokenServiceImpl) CreateApiToken(apiToken models.ApiToken) (models.ApiToken, error) {
	key, err := GenerateToken(40)
	if err != nil {
		return models.ApiToken{}, err
	}

	apiToken.Key = helpers.GenerateHMAC(u.config.APISecret, key)

	// if No value is passed, initialise to Zero value
	if apiToken.Expires == nil {
		apiToken.Expires = new(time.Time)
	}
	if apiToken.Enabled == nil {
		*apiToken.Enabled = true
	}

	res := u.db.Create(&apiToken)

	// Passing unencripted key on creation
	apiToken.Key = key

	return apiToken, res.Error
}

func (u *ApiTokenServiceImpl) UpdateApiToken(apiToken models.ApiToken) (models.ApiToken, error) {
	oldToken, err := u.GetApiToken(apiToken.ID.String())
	if err != nil {
		return models.ApiToken{}, err
	}

	if apiToken.Enabled != nil {
		oldToken.Enabled = apiToken.Enabled
	}
	if apiToken.Description != "" {
		oldToken.Description = apiToken.Description
	}
	if apiToken.Expires != nil {
		oldToken.Expires = apiToken.Expires
	}

	res := u.db.Updates(oldToken)
	if res.Error != nil {
		return models.ApiToken{}, res.Error
	}

	newToken, err := u.GetApiToken(apiToken.ID.String())
	if err != nil {
		return models.ApiToken{}, err
	}
	return newToken, nil
}

func (u *ApiTokenServiceImpl) DeleteApiToken(id string) error {
	apiToken, err := u.GetApiToken(id)
	if err != nil {
		return err
	}
	return u.db.Unscoped().Delete(&apiToken).Error
}

func GenerateToken(n int) (string, error) {
	// Removing 4 chars from the total length for "kri_" prefix
	n -= 4
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return "kri_" + string(ret), nil
}
