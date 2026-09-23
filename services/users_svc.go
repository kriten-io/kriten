package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	"golang.org/x/exp/slices"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService interface {
	ListUsers([]string, models.UserQueryParams) ([]models.User, int, error)
	GetUser(string) (models.User, error)
	CreateUser(models.User) (models.User, error)
	UpdateUser(models.User) (models.User, error)
	DeleteUser(string) error
	GetByUsernameAndProvider(string, string) (models.User, error)
	GetUserRoles(string) ([]models.Role, error)
}

type UserServiceImpl struct {
	db     *gorm.DB
	config config.Config
}

func NewUserService(database *gorm.DB, config config.Config) UserService {
	return &UserServiceImpl{
		db:     database,
		config: config,
	}
}

func (u *UserServiceImpl) ListUsers(authList []string, params models.UserQueryParams) ([]models.User, int, error) {
	var users []models.User
	var res *gorm.DB

	if len(authList) == 0 {
		return users, 0, nil
	}

	if slices.Contains(authList, "*") {
		res = u.db.Find(&users)
	} else {
		res = u.db.Find(&users, authList)
	}
	if res.Error != nil {
		return users, 0, fmt.Errorf("failed to get users: %w", res.Error)
	}

	var filtered []models.User
	for _, user := range users {
		if params.Name != "" && !strings.Contains(strings.ToLower(user.Username), strings.ToLower(params.Name)) {
			continue
		}
		user.Password = ""
		filtered = append(filtered, user)
	}

	total := len(filtered)

	if params.Limit > 0 {
		start := params.Offset
		if start > total {
			start = total
		}
		end := start + params.Limit
		if end > total {
			end = total
		}
		filtered = filtered[start:end]
	}

	return filtered, total, nil
}

func (u *UserServiceImpl) GetUser(id string) (models.User, error) {
	var user models.User
	res := u.db.Where("id = ?", id).Find(&user)
	if res.Error != nil {
		return models.User{}, fmt.Errorf("failed to get user '%s': %w", id, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.User{}, fmt.Errorf("user '%s': %w", id, ErrSvcObjNotFound)
	}
	user.Password = ""
	return user, nil
}

func (u *UserServiceImpl) CreateUser(user models.User) (models.User, error) {
	if user.Provider == "local" {
		password, err := HashPassword(user.Password)
		if err != nil {
			return models.User{}, fmt.Errorf("failed to generate hash for user password: %w", err)
		}
		user.Password = password
	}

	res := u.db.Create(&user)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return models.User{}, fmt.Errorf("failed to create user '%s': %w", user.Username, ErrSvcDBDuplicatedKey)
		}
		return models.User{}, fmt.Errorf("failed to create user '%s': %w", user.Username, res.Error)
	}

	return user, nil
}

func (u *UserServiceImpl) UpdateUser(user models.User) (models.User, error) {
	password, err := HashPassword(user.Password)
	if err != nil {
		return models.User{}, fmt.Errorf("failed to generate hash for user password: %w", err)
	}

	user.Password = password
	res := u.db.Updates(user)
	if res.Error != nil {
		return models.User{}, fmt.Errorf("failed to update user '%s': %w", user.ID.String(), res.Error)
	}

	newUser, err := u.GetUser(user.ID.String())
	if err != nil {
		return models.User{}, fmt.Errorf("failed to get user '%s': %w", user.ID.String(), err)
	}
	return newUser, nil
}

func (u *UserServiceImpl) DeleteUser(userID string) error {
	var userGroups []models.Group
	user, err := u.GetUser(userID)
	if err != nil {
		return fmt.Errorf("failed to get user '%s': %w", userID, err)
	}

	res := u.db.Model(&models.Group{}).Where("? = ANY(users)", userID).Find(&userGroups)
	if res.Error != nil {
		return fmt.Errorf("failed to get user '%s' groups: %w", userID, res.Error)
	}

	if len(userGroups) != 0 {
		return fmt.Errorf("cannot delete user: %w", ErrSvcUserGroupMembership)
	}

	var apiTokens []models.ApiToken
	res = u.db.Where("owner = ?", userID).Find(&apiTokens)

	if res.Error != nil {
		return fmt.Errorf("failed to get API tokens for user: %w", res.Error)
	}
	if res.RowsAffected != 0 {
		return fmt.Errorf("found user owned API tokens, delete those first: %w", ErrSvcObjInUse)
	}

	res = u.db.Unscoped().Delete(&user)
	if res.Error != nil {
		return fmt.Errorf("failed to delete user '%s': %w", userID, res.Error)
	}
	return nil
}

func (u *UserServiceImpl) GetByUsernameAndProvider(username string, provider string) (models.User, error) {
	var user models.User
	res := u.db.Where("username = ? AND provider = ?", username, provider).Find(&user)
	if res.Error != nil {
		return models.User{}, fmt.Errorf("failed to get user '%s' - provider '%s': %w", username, provider, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.User{}, fmt.Errorf("user '%s' - provider '%s': %w", username, provider, ErrSvcObjNotFound)
	}

	return user, nil
}

func (u *UserServiceImpl) GetUserRoles(userID string) ([]models.Role, error) {
	var roles []models.Role
	var userGroups []models.Group

	_, err := u.GetUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user '%s': %w", userID, err)
	}
	res := u.db.Model(&models.Group{}).Where("? = ANY(users)", userID).Find(&userGroups)

	if res.Error != nil {
		return []models.Role{}, fmt.Errorf("failed to get user '%s' rbac roles: %w", userID, res.Error)
	}

	for _, group := range userGroups {
		for _, roleID := range group.Role_IDs {
			var role models.Role
			res = u.db.Model(&models.Role{}).Where("id = ?", roleID).Find(&role)
			if res.Error != nil {
				return []models.Role{}, fmt.Errorf("failed to get user '%s' rbac roles: %w", userID, res.Error)
			}
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to generate hash from password: %w", err)
	}
	return string(bytes), nil
}
