package services

import (
	"fmt"
	"kriten/config"
	"kriten/models"

	"golang.org/x/exp/slices"

	"github.com/go-errors/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService interface {
	ListUsers([]string) ([]models.User, error)
	GetUser(string) (models.User, error)
	CreateUser(models.User) (models.User, error)
	UpdateUser(models.User) (models.User, error)
	DeleteUser(string) error
	GetByUsernameAndProvider(string, string) (models.User, error)
	GetUserRoles(string, string) ([]models.Role, error)
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

func (u *UserServiceImpl) ListUsers(authList []string) ([]models.User, error) {
	var users []models.User
	var res *gorm.DB

	if len(authList) == 0 {
		return users, nil
	}

	if slices.Contains(authList, "*") {
		res = u.db.Find(&users)
	} else {
		res = u.db.Find(&users, authList)
	}
	if res.Error != nil {
		return users, res.Error
	}

	return users, nil
}

func (u *UserServiceImpl) GetUser(id string) (models.User, error) {
	var user models.User
	res := u.db.Where("user_id = ?", id).Find(&user)
	if res.Error != nil {
		return models.User{}, res.Error
	}

	if user.Username == "" {
		return models.User{}, fmt.Errorf("user %s not found, please check uuid", id)
	}

	return user, nil
}

func (u *UserServiceImpl) CreateUser(user models.User) (models.User, error) {
	if user.Provider == "local" {
		password, err := HashPassword(user.Password)
		if err != nil {
			return models.User{}, err
		}
		user.Password = password
	}

	res := u.db.Create(&user)

	return user, res.Error
}

func (u *UserServiceImpl) UpdateUser(user models.User) (models.User, error) {
	password, err := HashPassword(user.Password)
	if err != nil {
		return models.User{}, err
	}

	user.Password = password
	res := u.db.Updates(user)
	if res.Error != nil {
		return models.User{}, res.Error
	}

	newUser, err := u.GetUser(user.ID.String())
	if err != nil {
		return models.User{}, err
	}
	return newUser, nil
}

func (u *UserServiceImpl) DeleteUser(id string) error {
	user, err := u.GetUser(id)
	if err != nil {
		return err
	}
	return u.db.Unscoped().Delete(&user).Error
}

func (u *UserServiceImpl) GetByUsernameAndProvider(username string, provider string) (models.User, error) {
	var user models.User
	res := u.db.Where("username = ? AND provider = ?", username, provider).Find(&user)
	if res.Error != nil {
		return models.User{}, res.Error
	}

	if user.Username == "" {
		return models.User{}, errors.New("user not found")
	}

	return user, nil
}

func (u *UserServiceImpl) GetUserRoles(subjectID string, provider string) ([]models.Role, error) {
	var roles []models.Role

	// SELECT *
	// FROM roles
	// INNER JOIN role_bindings
	// ON roles.role_id = role_bindings.role_id
	// WHERE role_bindings.subject_provider = provider AND role_bindings.subject_id = subjectID;
	res := u.db.Model(&models.Role{}).Joins(
		"left join role_bindings on roles.role_id = role_bindings.role_id").Where(
		"role_bindings.subject_provider = ? AND role_bindings.subject_id = ?", provider, subjectID).Find(&roles)
	if res.Error != nil {
		return []models.Role{}, res.Error
	}

	return roles, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
