package services

import (
	"errors"
	"fmt"
	"log"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type RoleBindingService interface {
	ListRoleBindings([]string, map[string]string) ([]models.RoleBinding, error)
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
	filters map[string]string,
) ([]models.RoleBinding, error) {
	var roleBindings []models.RoleBinding
	var res *gorm.DB

	if len(authList) == 0 {
		return roleBindings, nil
	} else if slices.Contains(authList, "*") {
		res = r.db.Where(filters).Find(&roleBindings)
	} else {
		res = r.db.Where(filters).Find(&roleBindings, authList)
	}
	if res.Error != nil {
		return roleBindings, res.Error
	}

	return roleBindings, nil
}

func (r *RoleBindingServiceImpl) GetRoleBinding(id string) (models.RoleBinding, error) {
	var role models.RoleBinding
	res := r.db.Where("name = ?", id).Find(&role)
	if res.Error != nil {
		return models.RoleBinding{}, res.Error
	}

	if role.Name == "" {
		return models.RoleBinding{}, fmt.Errorf("role_binding %s not found, please check id", id)
	}

	return role, nil
}

func (r *RoleBindingServiceImpl) CreateRoleBinding(roleBinding models.RoleBinding) (models.RoleBinding, error) {
	roleID, subjectID, err := r.CheckRoleBinding(roleBinding)
	if err != nil {
		return roleBinding, err
	}
	roleBinding.RoleID = roleID
	roleBinding.SubjectID = subjectID

	res := r.db.Create(&roleBinding)

	return roleBinding, res.Error
}

func (r *RoleBindingServiceImpl) UpdateRoleBinding(roleBinding models.RoleBinding) (models.RoleBinding, error) {
	roleID, subjectID, err := r.CheckRoleBinding(roleBinding)
	if err != nil {
		return roleBinding, err
	}
	roleBinding.RoleID = roleID
	roleBinding.SubjectID = subjectID
	res := r.db.Updates(roleBinding)
	if res.Error != nil {
		return models.RoleBinding{}, res.Error
	}

	newRoleBinding, err := r.GetRoleBinding(roleBinding.ID.String())
	if err != nil {
		return models.RoleBinding{}, err
	}
	return newRoleBinding, nil
}

func (r *RoleBindingServiceImpl) DeleteRoleBinding(id string) error {
	roleBinding, err := r.GetRoleBinding(id)
	if err != nil {
		return err
	}

	if roleBinding.Builtin {
		return errors.New("cannot delete builtin resource")
	}

	return r.db.Unscoped().Delete(&roleBinding).Error
}

func (r *RoleBindingServiceImpl) CheckRoleBinding(roleBinding models.RoleBinding) (uuid.UUID, uuid.UUID, error) {
	role, err := r.RoleService.GetRole(roleBinding.RoleName)
	if err != nil {
		log.Println(err)
		return uuid.UUID{}, uuid.UUID{}, err
	}

	var group models.Group
	if roleBinding.SubjectKind == "groups" {
		group, err = r.GroupService.GetGroup(roleBinding.SubjectName)
		if err != nil {
			log.Println(err)
			return uuid.UUID{}, uuid.UUID{}, err
		}
	} else {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("subject_kind not valid")
	}

	return role.ID, group.ID, nil
}
