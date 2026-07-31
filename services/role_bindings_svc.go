package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type RoleBindingService interface {
	ListRoleBindings([]string, models.RoleBindingQueryParams) ([]models.RoleBinding, error)
	GetRoleBinding(string) (models.RoleBinding, error)
	CreateRoleBinding(models.RoleBinding) (models.RoleBinding, error)
	UpdateRoleBinding(models.RoleBinding) (models.RoleBinding, error)
	DeleteRoleBinding(string) error
	CheckRoleBinding(models.RoleBinding) (uuid.UUID, uuid.UUID, error)
}

type RoleBindingServiceImpl struct {
	RoleService  RoleService
	GroupService GroupService
	db           *gorm.DB
	config       config.Config
}

func NewRoleBindingService(db *gorm.DB, config config.Config, rs RoleService, gs GroupService) RoleBindingService {
	return &RoleBindingServiceImpl{
		db:           db,
		config:       config,
		RoleService:  rs,
		GroupService: gs,
	}
}

func (r *RoleBindingServiceImpl) ListRoleBindings(
	authList []string,
	params models.RoleBindingQueryParams,
) ([]models.RoleBinding, error) {
	var roleBindings []models.RoleBinding
	var res *gorm.DB

	if len(authList) == 0 {
		return roleBindings, nil
	} else if slices.Contains(authList, "*") {
		res = r.db.Find(&roleBindings)
	} else {
		res = r.db.Find(&roleBindings, authList)
	}
	if res.Error != nil {
		return roleBindings, fmt.Errorf("error getting role bindings: %w", res.Error)
	}

	var filtered []models.RoleBinding
	for _, roleBinding := range roleBindings {
		if params.Name != "" && !strings.Contains(strings.ToLower(roleBinding.Name), strings.ToLower(params.Name)) {
			continue
		}
		filtered = append(filtered, roleBinding)
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

func (r *RoleBindingServiceImpl) GetRoleBinding(id string) (models.RoleBinding, error) {
	var roleBinding models.RoleBinding
	res := r.db.Where("id = ?", id).Find(&roleBinding)
	if res.Error != nil {
		return models.RoleBinding{}, fmt.Errorf("error getting role binding '%s': %w", id, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.RoleBinding{}, fmt.Errorf("error getting role binding '%s': %w", id, ErrSvcObjNotFound)
	}

	return roleBinding, nil
}

func (r *RoleBindingServiceImpl) CreateRoleBinding(roleBinding models.RoleBinding) (models.RoleBinding, error) {
	roleID, groupID, err := r.CheckRoleBinding(roleBinding)
	if err != nil {
		return models.RoleBinding{}, fmt.Errorf("role binding '%s': %w, %s", roleBinding.Name, ErrSvcRoleBindingValidation, err.Error())
	}
	roleBinding.RoleID = roleID
	roleBinding.GroupID = groupID

	res := r.db.Create(&roleBinding)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return models.RoleBinding{}, fmt.Errorf("error creating role binding'%s': %w", roleBinding.Name, ErrSvcDBDuplicatedKey)
		}
		return models.RoleBinding{}, fmt.Errorf("error creating role binding '%s': %w", roleBinding.Name, res.Error)
	}

	return roleBinding, res.Error
}

func (r *RoleBindingServiceImpl) UpdateRoleBinding(roleBinding models.RoleBinding) (models.RoleBinding, error) {

	_, err := r.GetRoleBinding(roleBinding.ID.String())
	if err != nil {
		return roleBinding, fmt.Errorf("error getting role binding: %w", err)
	}

	roleID, groupID, err := r.CheckRoleBinding(roleBinding)
	if err != nil {
		return models.RoleBinding{}, fmt.Errorf("role binding '%s': %w, %s", roleBinding.Name, ErrSvcRoleBindingValidation, err.Error())
	}
	roleBinding.RoleID = roleID
	roleBinding.GroupID = groupID
	res := r.db.Updates(roleBinding)
	if res.Error != nil {
		return models.RoleBinding{}, fmt.Errorf("error updating role binding '%s': %w", roleBinding.ID.String(), res.Error)
	}

	newRoleBinding, err := r.GetRoleBinding(roleBinding.ID.String())
	if err != nil {
		return models.RoleBinding{}, fmt.Errorf("error getting role binding '%s': %w", roleBinding.ID.String(), err)
	}
	return newRoleBinding, nil
}

func (r *RoleBindingServiceImpl) DeleteRoleBinding(id string) error {
	roleBinding, err := r.GetRoleBinding(id)
	if err != nil {
		return fmt.Errorf("error getting role binding: %w", err)
	}

	if roleBinding.Builtin {
		return fmt.Errorf("error deleting role binding '%s': %w", id, ErrSvcDeleteBuiltin)
	}

	res := r.db.Unscoped().Delete(&roleBinding)
	if res.Error != nil {
		return fmt.Errorf("error deleting role binding '%s': %w", id, res.Error)
	}
	return nil
}

func (r *RoleBindingServiceImpl) CheckRoleBinding(roleBinding models.RoleBinding) (uuid.UUID, uuid.UUID, error) {

	role, err := r.RoleService.GetRole(roleBinding.RoleID.String())
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("error getting role: %w", err)
	}

	var group models.Group

	group, err = r.GroupService.GetGroupByID(roleBinding.GroupID.String())
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("error getting group: %w", err)
	}

	return role.ID, group.ID, nil
}
