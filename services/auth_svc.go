package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	jwt "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

type AuthService interface {
	Login(*models.Credentials) (string, int, error)
	Refresh(string) (string, int, error)
	ChangePassword(string, *models.ChangePassword) error
	IsAutorised(*models.Authorization) (bool, error)
	GetAuthorizationList(*models.Authorization) ([]string, error)
	ValidateAPIToken(string) (models.User, error)
	ValidateWebhookSignatureInfraHub(string, string, string, string, []byte) (models.User, string, error)
	ValidateWebhookSignatureCommon(string, string, []byte) (models.User, string, error)
}

type AuthServiceImpl struct {
	db          *gorm.DB
	UserService UserService
	RoleService RoleService
	config      config.Config
}

func NewAuthService(
	config config.Config,
	us UserService,
	rls RoleService,
	database *gorm.DB,
) AuthService {
	return &AuthServiceImpl{
		config:      config,
		UserService: us,
		RoleService: rls,
		db:          database,
	}
}

// Login - TODO: This function is getting very crowded
// might need to be refactored in the future.
func (a *AuthServiceImpl) Login(credentials *models.Credentials) (string, int, error) {
	var user models.User
	var err error

	if credentials.Provider == "local" {
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

	claims.ExpiresAt = jwt.NewNumericDate(expirationTime)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err = token.SignedString(a.config.JWT.Key)
	if err != nil {
		log.Println(err)
		return "", -1, fmt.Errorf("failed to refresh token: %w", err)
	}

	return tokenStr, a.config.JWT.ExpirySeconds, nil
}

func (a *AuthServiceImpl) ChangePassword(tokenStr string, creds *models.ChangePassword) error {
	claims, err := helpers.ValidateJWTToken(tokenStr, a.config.JWT)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	if claims.Provider != "local" {
		return errors.New("password change is only allowed to local users.")
	}

	user, err := a.UserService.GetByUsernameAndProvider(claims.Username, claims.Provider)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.CurrentPassword))
	if err != nil {
		return errors.New("incorrect current password")
	}

	user.Password = creds.NewPassword

	_, err = a.UserService.UpdateUser(user)
	if err != nil {
		return fmt.Errorf("failed to update user %s password in DB: %w", user.Username, err)
	}

	return nil

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

func (a *AuthServiceImpl) ValidateWebhookSignatureInfraHub(
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
	data = append(data, body...)

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

func (a *AuthServiceImpl) ValidateWebhookSignatureCommon(
	id string,
	signature string,
	body []byte,
) (models.User, string, error) {
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

	h := hmac.New(sha512.New, []byte(webhook.Secret))
	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return models.User{}, "", errors.New("invalid signature")
	}

	var user models.User
	res = a.db.Where("id = ?", webhook.Owner).Find(&user)
	if res.Error != nil {
		return models.User{}, "", res.Error
	}
	return user, webhook.Task, nil
}

func (a *AuthServiceImpl) IsAutorised(auth *models.Authorization) (bool, error) {
	// Checking if the user owns the API token
	if auth.Resource == "apiTokens" {
		var apiToken models.ApiToken
		res := a.db.Where("id = ?", auth.ResourceName).Find(&apiToken)
		if res.Error != nil {
			return false, res.Error
		}

		if apiToken.Owner == auth.UserID {
			return true, nil
		}
	}

	roles, err := a.UserService.GetUserRoles(auth.UserID.String())
	if err != nil {
		return false, err
	}

	for i := range roles {
		role := &roles[i]
		if role.Resource == "*" || role.Resource == auth.Resource &&
			(len(role.Resource_Names) > 0 && slices.Contains(role.Resource_Names, "*") ||
				slices.Contains(role.Resource_Names, auth.ResourceName)) &&
			(role.Access == auth.Access || role.Access == "write") {
			return true, nil
		}
		// permission to run tasks gives also permissions to read associated jobs and tasks
		if (auth.Resource == "jobs" || auth.Resource == "tasks") && auth.Access == "read" &&
			slices.Contains(role.Resource_Names, auth.ResourceName) &&
			role.Resource == "tasks" && role.Access == "execute" {
			return true, nil
		}

	}

	return false, nil
}

func (a *AuthServiceImpl) GetAuthorizationList(auth *models.Authorization) ([]string, error) {
	roles, err := a.UserService.GetUserRoles(auth.UserID.String())
	if err != nil {
		log.Println(err)
		return []string{}, err
	}
	var authList []string
	for i := range roles {
		role := &roles[i]
		if role.Resource == "*" || role.Resource == auth.Resource {
			if slices.Contains(role.Resource_Names, "*") {
				return []string{"*"}, nil
			}
			authList = append(authList, role.Resource_Names...)
		}
		if auth.Resource == "jobs" {
			if role.Resource == "tasks" && role.Access == "execute" {
				authList = append(authList, role.Resource_Names...)
			}
		}
	}
	return authList, nil
}
