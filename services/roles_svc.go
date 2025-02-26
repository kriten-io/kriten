package services

import (
	"errors"
	"fmt"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type RoleService interface {
	ListRoles([]string) ([]models.Role, error)
	GetRole(string) (models.Role, error)
	CreateRole(models.Role) (models.Role, error)
	UpdateRole(models.Role) (models.Role, error)
	DeleteRole(string) error
}

type RoleServiceImpl struct {
	db                 *gorm.DB
	config             config.Config
	RoleBindingService *RoleBindingService
	UserService        *UserService
}

func NewRoleService(database *gorm.DB, config config.Config, rbs *RoleBindingService, us *UserService) RoleService {
	return &RoleServiceImpl{
		db:                 database,
		config:             config,
		RoleBindingService: rbs,
		UserService:        us,
	}
}

func (r *RoleServiceImpl) ListRoles(authList []string) ([]models.Role, error) {
	var roles []models.Role
	var res *gorm.DB

	if len(authList) == 0 {
		return roles, nil
	} else if slices.Contains(authList, "*") {
		res = r.db.Find(&roles)
	} else {
		res = r.db.Find(&roles, authList)
	}
	if res.Error != nil {
		return roles, res.Error
	}

	return roles, nil
}

func (r *RoleServiceImpl) GetRole(id string) (models.Role, error) {
	var role models.Role
	res := r.db.Where("name = ?", id).Find(&role)
	if res.Error != nil {
		return models.Role{}, res.Error
	}

	if role.Name == "" {
		return models.Role{}, fmt.Errorf("role %s not found, please check id", id)
	}

	return role, nil
}

func (r *RoleServiceImpl) CreateRole(role models.Role) (models.Role, error) {
	err := r.CheckRole(role)
	if err != nil {
		return role, err
	}
	res := r.db.Create(&role)

	return role, res.Error
}

func (r *RoleServiceImpl) UpdateRole(role models.Role) (models.Role, error) {
	err := r.CheckRole(role)
	if err != nil {
		return role, err
	}

	res := r.db.Updates(role)
	if res.Error != nil {
		return models.Role{}, res.Error
	}

	newRole, err := r.GetRole(role.ID.String())
	if err != nil {
		return models.Role{}, err
	}
	return newRole, nil
}

func (r *RoleServiceImpl) DeleteRole(id string) error {
	rbs := *r.RoleBindingService
	roleBindings, err := rbs.ListRoleBindings([]string{"*"}, nil)
	if err != nil {
		return err
	}
	for _, r := range roleBindings {
		if r.RoleID.String() == id {
			return fmt.Errorf("role is bound via role_binding: %s , please delete that first", r.ID)
		}
	}

	role, err := r.GetRole(id)
	if err != nil {
		return err
	}

	if role.Buitin {
		return errors.New("cannot delete builtin resource")
	}

	return r.db.Unscoped().Delete(&role).Error
}

// TODO: This is very crowded and repetitive
// might need a refactor in the future
func (r *RoleServiceImpl) CheckRole(role models.Role) error {
	if role.Resource == "users" {
		for _, user := range role.Resources_IDs {
			us := *r.UserService
			_, err := us.GetUser(user)
			if err != nil {
				return err
			}
		}
	} else if role.Resource == "roles" {
		for _, role := range role.Resources_IDs {
			_, err := r.GetRole(role)
			if err != nil {
				return err
			}
		}
	} else if role.Resource == "role_bindings" {
		for _, roleBindings := range role.Resources_IDs {
			rbs := *r.RoleBindingService
			_, err := rbs.GetRoleBinding(roleBindings)
			if err != nil {
				return err
			}
		}
	} else {
		for _, c := range role.Resources_IDs {
			configMap, err := helpers.GetConfigMap(r.config.Kube, c)
			if err != nil {
				return err
			}
			// configmaps are all stored in the same namespace, so we need to identify the resource
			if role.Resource == "runners" {
				if configMap.Data["image"] == "" {
					return fmt.Errorf("runner %s not found", c)
				}
			} else if role.Resource == "tasks" {
				if configMap.Data["runner"] == "" {
					return fmt.Errorf("task %s not found", c)
				}
			} else if role.Resource == "jobs" {
				if configMap.Data["runner"] == "" {
					return fmt.Errorf("job %s not found", c)
				}
			}
		}
	}

	return nil
}
