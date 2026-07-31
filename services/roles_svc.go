package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"

	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type RoleService interface {
	ListRoles([]string, models.RoleQueryParams) ([]models.Role, error)
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

func (r *RoleServiceImpl) ListRoles(authList []string, params models.RoleQueryParams) ([]models.Role, error) {
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
		return roles, fmt.Errorf("error getting list of roles: %w", res.Error)
	}

	var filtered []models.Role
	for _, role := range roles {
		if params.Name != "" && !strings.Contains(strings.ToLower(role.Name), strings.ToLower(params.Name)) {
			continue
		}
		filtered = append(filtered, role)
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

	return filtered, nil
}

func (r *RoleServiceImpl) GetRole(id string) (models.Role, error) {
	var role models.Role
	res := r.db.Where("id = ?", id).Find(&role)
	if res.Error != nil {
		return models.Role{}, fmt.Errorf("error getting role '%s': %w", id, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.Role{}, fmt.Errorf("role '%s': %w", id, ErrSvcObjNotFound)
	}

	return role, nil
}

func (r *RoleServiceImpl) CreateRole(role models.Role) (models.Role, error) {
	err := r.CheckRole(role)
	if err != nil {
		return models.Role{}, fmt.Errorf("role '%s': %w, %s", role.Name, ErrSvcRoleValidation, err.Error())
	}
	res := r.db.Create(&role)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return models.Role{}, fmt.Errorf("error creating role '%s': %w", role.Name, ErrSvcDBDuplicatedKey)
		}
		return models.Role{}, fmt.Errorf("error creating role '%s': %w", role.Name, res.Error)
	}

	return role, nil
}

func (r *RoleServiceImpl) UpdateRole(role models.Role) (models.Role, error) {

	_, err := r.GetRole(role.ID.String())
	if err != nil {
		return models.Role{}, fmt.Errorf("error getting role: %w", err)
	}

	err = r.CheckRole(role)
	if err != nil {
		return models.Role{}, fmt.Errorf("role '%s': %w, %s", role.Name, ErrSvcRoleValidation, err.Error())
	}

	res := r.db.Updates(role)
	if res.Error != nil {
		return models.Role{}, fmt.Errorf("error updating role '%s': %w", role.ID.String(), res.Error)
	}

	newRole, err := r.GetRole(role.ID.String())
	if err != nil {
		return models.Role{}, fmt.Errorf("error getting role: %w", err)
	}
	return newRole, nil
}

func (r *RoleServiceImpl) DeleteRole(id string) error {
	role, err := r.GetRole(id)
	if err != nil {
		return fmt.Errorf("error getting role: %w", err)
	}

	if role.Builtin {
		return fmt.Errorf("error deleting role '%s': %w", id, ErrSvcDeleteBuiltin)
	}

	rbs := *r.RoleBindingService
	var params models.RoleBindingQueryParams
	roleBindings, err := rbs.ListRoleBindings([]string{"*"}, params)
	if err != nil {
		return fmt.Errorf("error getting role bindings: %w", err)
	}

	for _, r := range roleBindings {
		if r.RoleID.String() == id {
			return fmt.Errorf("role is bound via role_binding: %s , please delete that first: %w", r.ID, ErrSvcObjInUse)
		}
	}

	res := r.db.Unscoped().Delete(&role)
	if res.Error != nil {
		return fmt.Errorf("error deleting role '%s': %w", id, res.Error)
	}

	return nil
}

// TODO: This is very crowded and repetitive
// might need a refactor in the future.
func (r *RoleServiceImpl) CheckRole(role models.Role) error {

	for _, c := range role.Resource_IDs {
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

	return nil
}
