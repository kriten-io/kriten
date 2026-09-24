package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type ApiTokenService interface {
	ListApiTokens(uuid.UUID) ([]models.ApiToken, error)
	ListAllApiTokens([]string) ([]models.ApiToken, error)
	GetApiToken(string) (models.ApiToken, error)
	CreateApiToken(models.Actor, models.ApiToken) (models.ApiToken, error)
	UpdateApiToken(models.Actor, models.ApiToken) (models.ApiToken, error)
	DeleteApiToken(models.Actor, string) error
}

const tokensCategory = "apiTokens"

type ApiTokenServiceImpl struct {
	db     *gorm.DB
	config config.Config
	audit  AuditService
}

func NewApiTokenService(database *gorm.DB, config config.Config, als AuditService) ApiTokenService {
	return &ApiTokenServiceImpl{
		db:     database,
		config: config,
		audit:  als,
	}
}

func (u *ApiTokenServiceImpl) ListApiTokens(userid uuid.UUID) ([]models.ApiToken, error) {
	var apiTokens []models.ApiToken

	res := u.db.Select("id", "owner", "expires", "created_at", "updated_at", "description", "enabled").
		Where("owner = ?", userid).
		Find(&apiTokens)

	if res.Error != nil {
		return apiTokens, fmt.Errorf("error getting user API tokens: %w", res.Error)
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
		return apiTokens, fmt.Errorf("error getting all API tokens: %w", res.Error)
	}

	return apiTokens, nil
}

func (u *ApiTokenServiceImpl) GetApiToken(id string) (models.ApiToken, error) {
	var apiToken models.ApiToken

	res := u.db.Select("id", "owner", "expires", "created_at", "updated_at", "description", "enabled").
		Where("id = ?", id).
		Find(&apiToken)

	if res.Error != nil {
		return models.ApiToken{}, fmt.Errorf("error getting API token '%s': %w", id, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.ApiToken{}, fmt.Errorf("API token '%s': %w", id, ErrSvcObjNotFound)
	}

	return apiToken, nil
}

func (u *ApiTokenServiceImpl) CreateApiToken(actor models.Actor, apiToken models.ApiToken) (models.ApiToken, error) {
	audit := u.audit.NewAuditLog(actor, "create", tokensCategory, apiToken.Key)

	key, err := GenerateToken(40)
	if err != nil {
		u.audit.CreateAudit(audit)
		return models.ApiToken{}, fmt.Errorf("error generating API token: %w", err)
	}
	var tokenEnabled = true
	apiToken.Key = helpers.GenerateHMAC(u.config.APISecret, key)

	// if No value is passed, initialise to Zero value
	if apiToken.Expires == nil {
		apiToken.Expires = new(time.Time)
	}

	// nil pointer dereference via tokenEnabled bool var
	if apiToken.Enabled == nil {
		apiToken.Enabled = &tokenEnabled
	}

	res := u.db.Create(&apiToken)
	if res.Error != nil {
		u.audit.CreateAudit(audit)
		return models.ApiToken{}, fmt.Errorf("error creating API token: %w", res.Error)
	}

	// Passing unencripted key on creation
	apiToken.Key = key
	audit.EventTarget = apiToken.Key

	audit.Status = "success"
	u.audit.CreateAudit(audit)
	return apiToken, nil
}

func (u *ApiTokenServiceImpl) UpdateApiToken(actor models.Actor, apiToken models.ApiToken) (models.ApiToken, error) {
	audit := u.audit.NewAuditLog(actor, "update", tokensCategory, apiToken.ID.String())

	oldToken, err := u.GetApiToken(apiToken.ID.String())
	if err != nil {
		u.audit.CreateAudit(audit)
		return models.ApiToken{}, fmt.Errorf("error getting API token: %w", err)
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
		u.audit.CreateAudit(audit)
		return models.ApiToken{}, fmt.Errorf("error updating API token: %w", res.Error)
	}

	newToken, err := u.GetApiToken(apiToken.ID.String())
	if err != nil {
		u.audit.CreateAudit(audit)
		return models.ApiToken{}, fmt.Errorf("error getting token: %w", err)
	}
	audit.Status = "success"
	u.audit.CreateAudit(audit)
	return newToken, nil
}

func (u *ApiTokenServiceImpl) DeleteApiToken(actor models.Actor, id string) error {
	audit := u.audit.NewAuditLog(actor, "delete", tokensCategory, id)

	apiToken, err := u.GetApiToken(id)
	if err != nil {
		u.audit.CreateAudit(audit)
		return fmt.Errorf("error getting API token: %w", err)
	}

	res := u.db.Unscoped().Delete(&apiToken)
	if res.Error != nil {
		u.audit.CreateAudit(audit)
		return fmt.Errorf("error deleting API token '%s': %w", id, res.Error)
	}
	audit.Status = "success"
	u.audit.CreateAudit(audit)
	return nil
}

func GenerateToken(n int) (string, error) {
	// Removing 4 chars from the total length for "kri_" prefix
	n -= 4
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", fmt.Errorf("error generating new token: %w", err)
		}
		ret[i] = letters[num.Int64()]
	}

	return "kri_" + string(ret), nil
}
