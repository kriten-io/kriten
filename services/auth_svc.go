package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

type AuthService interface {
	Login(*models.Credentials) (string, int, error)
	Refresh(string) (string, int, error)
	IsAutorised(*models.Authorization) (bool, error)
	GetAuthorizationList(*models.Authorization) ([]string, error)
	ValidateAPIToken(string) (models.User, error)
	ValidateWebhookSignature(string, string, string, string, []byte) (models.User, string, error)
}

type AuthServiceImpl struct {
	db                 *gorm.DB
	UserService        UserService
	RoleService        RoleService
	RoleBindingService RoleBindingService
	config             config.Config
}

func NewAuthService(
	config config.Config,
	us UserService,
	rls RoleService,
	rbc RoleBindingService,
	database *gorm.DB,
) AuthService {
	return &AuthServiceImpl{
		config:             config,
		UserService:        us,
		RoleService:        rls,
		RoleBindingService: rbc,
		db:                 database,
	}
}

// Login - TODO: This function is getting very crowded
// might need to be refactored in the future.
func (a *AuthServiceImpl) Login(credentials *models.Credentials) (string, int, error) {
	var user models.User
	var err error

	if credentials.Username == "root" {
		rootPassword, err := a.GetRootPassword()
		if err != nil {
			return "", -1, fmt.Errorf("failed to get root password: %w", err)
		}
		if credentials.Password != rootPassword {
			err := errors.New("password is incorrect")
			return "", -1, fmt.Errorf("failed to authenticate: %w", err)
		}
		user, err = a.UserService.GetByUsernameAndProvider(credentials.Username, credentials.Provider)
		if err != nil {
			return "", -1, fmt.Errorf("user not found: %w", err)
		}
	} else if credentials.Provider == "local" {
		user, err = a.UserService.GetByUsernameAndProvider(credentials.Username, credentials.Provider)
		if err != nil {
			return "", -1, fmt.Errorf("user not found: %w", err)
		}
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(credentials.Password))
		if err != nil {
			return "", -1, fmt.Errorf("incorrect password: %w", err)
		}
	} else if credentials.Provider == "active_directory" {
		err := helpers.BindAndSearch(a.config.LDAP, credentials.Username, credentials.Password)
		if err != nil {
			return "", -1, fmt.Errorf("failed to authenticate: %w", err)
		}
		_, err = a.UserService.CreateUser(models.User{
			Username: credentials.Username,
			Provider: credentials.Provider,
		})
		if err != nil && !strings.Contains(err.Error(), "ERROR: duplicate key value violates unique constraint") {
			log.Println(err.Error())
			return "", -1, fmt.Errorf("failed to create ldap user into local user db: %w", err)
		}
		user, err = a.UserService.GetByUsernameAndProvider(credentials.Username, credentials.Provider)
		if err != nil {
			return "", -1, fmt.Errorf("failed to get user credentials: %w", err)
		}
	} else {
		err := errors.New("provider does not exist")
		return "", -1, fmt.Errorf("unknown provider: %w", err)
	}

	token, err := helpers.CreateJWTToken(credentials, user.ID, a.config.JWT)
	if err != nil {
		log.Println(err)
		return "", -1, fmt.Errorf("failed to create token: %w", err)
	}

	return token, a.config.JWT.ExpirySeconds, nil
}

func (a *AuthServiceImpl) Refresh(tokenStr string) (string, int, error) {
	claims, err := helpers.ValidateJWTToken(tokenStr, a.config.JWT)
	if err != nil {
		return "", -1, fmt.Errorf("failed to validate token: %w", err)
	}

	expirationTime := time.Now().Add(time.Second * time.Duration(a.config.JWT.ExpirySeconds))

	claims.ExpiresAt = expirationTime.Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err = token.SignedString(a.config.JWT.Key)
	if err != nil {
		log.Println(err)
		return "", -1, fmt.Errorf("failed to refresh token: %w", err)
	}

	return tokenStr, a.config.JWT.ExpirySeconds, nil
}

func (a *AuthServiceImpl) GetRootPassword() (string, error) {
	secret, err := helpers.GetSecret(a.config.Kube, a.config.RootSecret)

	if err != nil {
		return "", fmt.Errorf("failed to get secret for root user: %w", err)
	}

	password := secret.Data["password"]

	return string(password), nil
}

func (a *AuthServiceImpl) ValidateAPIToken(key string) (models.User, error) {
	var apiToken models.ApiToken
	apiKey := helpers.GenerateHMAC(a.config.APISecret, key)

	res := a.db.Where("key = ?", apiKey).Find(&apiToken)
	if res.Error != nil {
		return models.User{}, res.Error
	}

	// checking if there's any result
	if res.RowsAffected == 0 {
		return models.User{}, errors.New("invalid token")
	}

	if !apiToken.Expires.IsZero() && apiToken.Expires.Before(time.Now()) {
		return models.User{}, errors.New("token expired")
	}
	if !*apiToken.Enabled {
		return models.User{}, errors.New("token not enabled")
	}

	// Token is Valid, retrieving User info
	var user models.User
	res = a.db.Where("user_id = ?", apiToken.Owner).Find(&user)
	if res.Error != nil {
		return models.User{}, res.Error
	}
	return user, nil
}

func (a *AuthServiceImpl) ValidateWebhookSignature(
	id string,
	msgID string,
	msgTimestamp string,
	signature string,
	body []byte,
) (models.User, string, error) {
	// Splitting the signature to get the to remove prepended "v1," from InfraHub
	split := strings.Split(signature, ",")
	if len(split) != 2 {
		return models.User{}, "", errors.New("invalid signature")
	}
	signature = split[1]
	data := []byte(fmt.Sprintf("%s.%s.", msgID, msgTimestamp))
	newBody := bytes.ReplaceAll(body, []byte(`"`), []byte(`'`))
	data = append(data, newBody...)

	var webhook models.Webhook
	res := a.db.Where("id = ?", id).Find(&webhook)
	if res.Error != nil {
		return models.User{}, "", res.Error
	}
	// checking if there's any result
	if res.RowsAffected == 0 {
		return models.User{}, "", errors.New("invalid webhook")
	}
	// checking if the signature is valid
	// Validating the signature

	h := hmac.New(sha256.New, []byte(webhook.Secret))
	h.Write(data)
	expectedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return models.User{}, "", errors.New("invalid signature")
	}

	var user models.User
	res = a.db.Where("user_id = ?", webhook.Owner).Find(&user)
	if res.Error != nil {
		return models.User{}, "", res.Error
	}
	return user, webhook.Task, nil
}

func (a *AuthServiceImpl) IsAutorised(auth *models.Authorization) (bool, error) {
	// Checking if the user owns the API token
	if auth.Resource == "apiTokens" {
		var apiToken models.ApiToken
		res := a.db.Where("id = ?", auth.ResourceID).Find(&apiToken)
		if res.Error != nil {
			return false, res.Error
		}

		if apiToken.Owner == auth.UserID {
			return true, nil
		}
	}

	roles, err := a.UserService.GetUserRoles(auth.UserID.String(), auth.Provider)
	if err != nil {
		log.Println(err)
		return false, err
	}
	for _, role := range roles {
		if role.Resource == "*" || role.Resource == auth.Resource &&
			(len(role.Resources_IDs) > 0 && role.Resources_IDs[0] == "*" ||
				slices.Contains(role.Resources_IDs, auth.ResourceID)) &&
			(role.Access == auth.Access || role.Access == "write") {
			return true, nil
		}
	}

	return false, nil
}

func (a *AuthServiceImpl) GetAuthorizationList(auth *models.Authorization) ([]string, error) {
	roles, err := a.UserService.GetUserRoles(auth.UserID.String(), auth.Provider)
	if err != nil {
		log.Println(err)
		return []string{}, err
	}

	var authList []string
	for i := range roles {
		role := &roles[i]
		if role.Resource == "*" || role.Resource == auth.Resource {
			if role.Resources_IDs[0] == "*" {
				return []string{"*"}, nil
			}
			authList = append(authList, role.Resources_IDs...)
		}
	}

	return authList, nil
}
