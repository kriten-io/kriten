package services

import (
	"errors"
	"kriten-core/config"
	"kriten-core/helpers"
	"kriten-core/models"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slices"
)

type AuthService interface {
	Login(*models.Credentials) (string, int, error)
	Refresh(string) (string, int, error)
	IsAutorised(*models.Authorization) (bool, error)
	GetAuthorizationList(*models.Authorization) ([]string, error)
}

type AuthServiceImpl struct {
	config             config.Config
	UserService        UserService
	RoleService        RoleService
	RoleBindingService RoleBindingService
}

func NewAuthService(config config.Config, us UserService, rls RoleService, rbc RoleBindingService) AuthService {
	return &AuthServiceImpl{
		config:             config,
		UserService:        us,
		RoleService:        rls,
		RoleBindingService: rbc,
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
			err := errors.New("Passowrd is incorrect")
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
	} else if !a.config.CommunityRelease && credentials.Provider == "active_directory" {
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

	token, err := helpers.CreateToken(credentials, user.ID, a.config.JWT)
	if err != nil {
		log.Println(err)
		return "", -1, err
	}

	return token, a.config.JWT.ExpirySeconds, nil
}

func (a *AuthServiceImpl) Refresh(tokenStr string) (string, int, error) {
	claims, err := helpers.ValidateToken(tokenStr, a.config.JWT)
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

func (a *AuthServiceImpl) IsAutorised(auth *models.Authorization) (bool, error) {
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
