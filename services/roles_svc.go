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
	ListRoles([]string, models.RoleQueryParams) ([]models.Role, int, error)
	GetRole(string) (models.Role, error)
	CreateRole(models.Actor, models.Role) (models.Role, error)
	UpdateRole(models.Actor, models.Role) (models.Role, error)
	DeleteRole(models.Actor, string) error
	ListRoleGroups(string) ([]models.RoleGroup, error)
}

const rolesCategory = "roles"

type RoleServiceImpl struct {
	db           *gorm.DB
	config       config.Config
	GroupService GroupService
	audit        AuditService
}

func NewRoleService(database *gorm.DB, config config.Config, gs GroupService, als AuditService) RoleService {
	return &RoleServiceImpl{
		db:           database,
		config:       config,
		GroupService: gs,
		audit:        als,
	}
}

func (r *RoleServiceImpl) ListRoles(authList []string, params models.RoleQueryParams) ([]models.Role, int, error) {
	var roles []models.Role
	var res *gorm.DB

	if len(authList) == 0 {
		return roles, 0, nil
	} else if slices.Contains(authList, "*") {
		res = r.db.Find(&roles)
	} else {
		res = r.db.Find(&roles, authList)
	}
	if res.Error != nil {
		return roles, 0, fmt.Errorf("error getting list of roles: %w", res.Error)
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

	return filtered, total, nil
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

func (r *RoleServiceImpl) CreateRole(actor models.Actor, role models.Role) (models.Role, error) {
	audit := r.audit.NewAuditLog(actor, "create", rolesCategory, role.Name)

	err := r.CheckRole(role)
	if err != nil {
		r.audit.CreateAudit(audit)
		return models.Role{}, fmt.Errorf("role '%s': %w, %s", role.Name, ErrSvcRoleValidation, err.Error())
	}
	res := r.db.Create(&role)
	if res.Error != nil {
		r.audit.CreateAudit(audit)
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return models.Role{}, fmt.Errorf("error creating role '%s': %w", role.Name, ErrSvcDBDuplicatedKey)
		}
		return models.Role{}, fmt.Errorf("error creating role '%s': %w", role.Name, res.Error)
	}

	audit.Status = "success"
	r.audit.CreateAudit(audit)
	return role, nil
}

func (r *RoleServiceImpl) UpdateRole(actor models.Actor, role models.Role) (models.Role, error) {
	audit := r.audit.NewAuditLog(actor, "update", rolesCategory, role.ID.String())

	_, err := r.GetRole(role.ID.String())
	if err != nil {
		r.audit.CreateAudit(audit)
		return models.Role{}, fmt.Errorf("error getting role: %w", err)
	}

	err = r.CheckRole(role)
	if err != nil {
		r.audit.CreateAudit(audit)
		return models.Role{}, fmt.Errorf("role '%s': %w, %s", role.Name, ErrSvcRoleValidation, err.Error())
	}

	res := r.db.Updates(role)
	if res.Error != nil {
		r.audit.CreateAudit(audit)
		return models.Role{}, fmt.Errorf("error updating role '%s': %w", role.ID.String(), res.Error)
	}

	newRole, err := r.GetRole(role.ID.String())
	if err != nil {
		r.audit.CreateAudit(audit)
		return models.Role{}, fmt.Errorf("error getting role: %w", err)
	}
	audit.Status = "success"
	r.audit.CreateAudit(audit)
	return newRole, nil
}

func (r *RoleServiceImpl) DeleteRole(actor models.Actor, id string) error {
	audit := r.audit.NewAuditLog(actor, "delete", rolesCategory, id)

	var groups []models.Group
	role, err := r.GetRole(id)
	if err != nil {
		r.audit.CreateAudit(audit)
		return fmt.Errorf("error getting role: %w", err)
	}

	if role.Builtin {
		r.audit.CreateAudit(audit)
		return fmt.Errorf("error deleting role '%s': %w", id, ErrSvcDeleteBuiltin)
	}

	res := r.db.Model(&models.Group{}).Where("? = ANY(roles)", id).Find(&groups)
	if len(groups) != 0 {
		r.audit.CreateAudit(audit)
		return fmt.Errorf("role %s is used, please remove role from groups first: %w", role.ID, ErrSvcObjInUse)
	}

	res = r.db.Unscoped().Delete(&role)
	if res.Error != nil {
		r.audit.CreateAudit(audit)
		return fmt.Errorf("error deleting role '%s': %w", id, res.Error)
	}
	audit.Status = "success"
	r.audit.CreateAudit(audit)
	return nil
}

// TODO: This is very crowded and repetitive
// might need a refactor in the future.
func (r *RoleServiceImpl) CheckRole(role models.Role) error {

	for _, c := range role.Resource_Names {
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

func (r *RoleServiceImpl) AddGroupsToRole(id string, groups []models.RoleGroup) (models.Role, error) {
	role, err := r.GetRole(id)
	if err != nil {
		return models.Role{}, fmt.Errorf("failed to get role: %w", err)
	}

	return role, nil
}

func (r *RoleServiceImpl) ListRoleGroups(id string) ([]models.RoleGroup, error) {
	var roleGroups []models.RoleGroup
	var groups []models.Group

	_, err := r.GetRole(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	res := r.db.Model(&models.Group{}).Where("? = ANY(roles)", id).Find(&groups)
	if res.Error != nil {
		return []models.RoleGroup{}, fmt.Errorf("failed to get role '%s' groups: %w", id, res.Error)
	}

	for _, group := range groups {
		roleGroups = append(roleGroups, models.RoleGroup{
			Group:    group.Name,
			Provider: group.Provider,
			ID:       group.ID,
		})

	}

	return roleGroups, nil
}
