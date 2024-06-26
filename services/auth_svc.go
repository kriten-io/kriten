package services

import (
	"errors"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"log"
	"strings"
	"time"

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
}

type AuthServiceImpl struct {
	config             config.Config
	UserService        UserService
	RoleService        RoleService
	RoleBindingService RoleBindingService
	db                 *gorm.DB
}

func NewAuthService(config config.Config, us UserService, rls RoleService, rbc RoleBindingService, database *gorm.DB) AuthService {
	return &AuthServiceImpl{
		config:             config,
		UserService:        us,
		RoleService:        rls,
		RoleBindingService: rbc,
		db:                 database,
	}
}

// TODO: This function is getting very crowded
// might need to be refactored in the future
func (a *AuthServiceImpl) Login(credentials *models.Credentials) (string, int, error) {
	var user models.User
	var err error

	if credentials.Username == "root" {
		rootPassword, err := a.GetRootPassword()
		if err != nil {
			return "", -1, err
		}
		if credentials.Password != rootPassword {
			err := errors.New("password is incorrect")
			return "", -1, err
		}
		user, err = a.UserService.GetByUsernameAndProvider(credentials.Username, credentials.Provider)
		if err != nil {
			return "", -1, err
		}
	} else if credentials.Provider == "local" {
		user, err = a.UserService.GetByUsernameAndProvider(credentials.Username, credentials.Provider)
		if err != nil {
			return "", -1, err
		}
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(credentials.Password))
		if err != nil {
			log.Println(err)
			return "", -1, err
		}
	} else if credentials.Provider == "active_directory" {
		err := helpers.BindAndSearch(a.config.LDAP, credentials.Username, credentials.Password)
		if err != nil {
			return "", -1, err
		}
		user, err = a.UserService.CreateUser(models.User{
			Username: credentials.Username,
			Provider: credentials.Provider,
		})
		if err != nil && !strings.Contains(err.Error(), "ERROR: duplicate key value violates unique constraint") {
			log.Println(err.Error())
			return "", -1, err
		}
		user, err = a.UserService.GetByUsernameAndProvider(credentials.Username, credentials.Provider)
		if err != nil {
			return "", -1, err
		}
	} else {
		err := errors.New("provider does not exist")
		return "", -1, err
	}

	token, err := helpers.CreateJWTToken(credentials, user.ID, a.config.JWT)
	if err != nil {
		log.Println(err)
		return "", -1, err
	}

	return token, a.config.JWT.ExpirySeconds, nil
}

func (a *AuthServiceImpl) Refresh(tokenStr string) (string, int, error) {
	claims, err := helpers.ValidateJWTToken(tokenStr, a.config.JWT)
	if err != nil {
		return "", -1, err
	}

	expirationTime := time.Now().Add(time.Second * time.Duration(a.config.JWT.ExpirySeconds))

	claims.ExpiresAt = expirationTime.Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err = token.SignedString(a.config.JWT.Key)
	if err != nil {
		log.Println(err)
		return "", -1, err
	}

	return tokenStr, a.config.JWT.ExpirySeconds, nil
}

func (a *AuthServiceImpl) GetRootPassword() (string, error) {
	secret, err := helpers.GetSecret(a.config.Kube, a.config.RootSecret)

	if err != nil {
		return "", err
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
			(len(role.Resources_IDs) > 0 && role.Resources_IDs[0] == "*" || slices.Contains(role.Resources_IDs, auth.ResourceID)) &&
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
	for _, role := range roles {
		if role.Resource == "*" || role.Resource == auth.Resource {
			if role.Resources_IDs[0] == "*" {
				return []string{"*"}, nil
			}
			authList = append(authList, role.Resources_IDs...)
		}
	}

	return authList, nil
}
