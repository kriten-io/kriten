package services

import (
	"kriten/config"
	"kriten/models"

	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type ApiTokenService interface {
	ListApiTokens([]string) ([]models.ApiToken, error)
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

func (u *ApiTokenServiceImpl) ListApiTokens(authList []string) ([]models.ApiToken, error) {
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
	res := u.db.Where("apiToken_id = ?", id).Find(&apiToken)
	if res.Error != nil {
		return models.ApiToken{}, res.Error
	}

	return apiToken, nil
}

func (u *ApiTokenServiceImpl) CreateApiToken(apiToken models.ApiToken) (models.ApiToken, error) {

	res := u.db.Create(&apiToken)

	return apiToken, res.Error
}

func (u *ApiTokenServiceImpl) UpdateApiToken(apiToken models.ApiToken) (models.ApiToken, error) {
	// password, err := HashPassword(apiToken.Password)
	// if err != nil {
	// 	return models.ApiToken{}, err
	// }

	// apiToken.Password = password
	// res := u.db.Updates(apiToken)
	// if res.Error != nil {
	// 	return models.ApiToken{}, res.Error
	// }

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
