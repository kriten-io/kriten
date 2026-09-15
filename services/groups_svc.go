package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type GroupService interface {
	ListGroups([]string, models.GroupQueryParams) ([]models.Group, error)
	GetGroup(string) (models.Group, error)
	GetUserGroups(string) ([]models.UserGroup, error)
	GetGroupByID(string) (models.Group, error)
	CreateGroup(models.Group) (models.Group, error)
	UpdateGroup(models.Group) (models.Group, error)
	ListGroupUsers(string) ([]models.GroupUser, error)
	AddUsersToGroup(string, []models.GroupUser) (models.Group, error)
	RemoveUsersFromGroup(string, []models.GroupUser) (models.Group, error)
	DeleteGroup(string) error
	GetGroupRoles(string) ([]models.Role, error)
	AddRolesToGroup(string, []models.GroupRole) (models.Group, error)
	RemoveRolesFromGroup(string, []models.GroupRole) (models.Group, error)
	ListGroupRoles(string) ([]models.GroupRole, error)
}

type GroupServiceImpl struct {
	db          *gorm.DB
	UserService UserService
	config      config.Config
}

func NewGroupService(database *gorm.DB, us UserService, config config.Config) GroupService {
	return &GroupServiceImpl{
		db:          database,
		UserService: us,
		config:      config,
	}
}

func (g *GroupServiceImpl) ListGroups(authList []string, params models.GroupQueryParams) ([]models.Group, error) {
	var groups []models.Group
	var res *gorm.DB

	if len(authList) == 0 {
		return groups, nil
	}

	if slices.Contains(authList, "*") {
		res = g.db.Find(&groups)
	} else {
		res = g.db.Find(&groups, authList)
	}
	if res.Error != nil {
		return groups, fmt.Errorf("groups: %w", res.Error)
	}

	var filtered []models.Group
	for _, group := range groups {
		if params.Name != "" && !strings.Contains(strings.ToLower(group.Name), strings.ToLower(params.Name)) {
			continue
		}
		filtered = append(filtered, group)
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

func (g *GroupServiceImpl) GetGroup(name string) (models.Group, error) {
	var group models.Group
	res := g.db.Where("name = ?", name).Find(&group)
	if res.Error != nil {
		return models.Group{}, fmt.Errorf("group '%s': %w", name, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.Group{}, fmt.Errorf("group '%s': %w", name, ErrSvcObjNotFound)
	}

	return group, nil
}

func (g *GroupServiceImpl) GetGroupByID(id string) (models.Group, error) {
	var group models.Group
	res := g.db.Where("id = ?", id).Find(&group)
	if res.Error != nil {
		return models.Group{}, fmt.Errorf("group '%s': %w", id, res.Error)
	}

	if res.RowsAffected == 0 {
		return models.Group{}, fmt.Errorf("group '%s': %w", id, ErrSvcObjNotFound)
	}

	return group, nil
}

func (g *GroupServiceImpl) CreateGroup(group models.Group) (models.Group, error) {
	res := g.db.Create(&group)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return models.Group{}, fmt.Errorf("error creating group '%s': %w", group.Name, ErrSvcDBDuplicatedKey)
		}
		return models.Group{}, fmt.Errorf("error creating group '%s': %w", group.Name, res.Error)
	}

	return group, nil
}

func (g *GroupServiceImpl) UpdateGroup(group models.Group) (models.Group, error) {
	res := g.db.Updates(group)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return models.Group{}, fmt.Errorf("group '%s': %w", group.Name, ErrSvcObjNotFound)
		}
		return models.Group{}, fmt.Errorf("group '%s': %w", group.Name, res.Error)
	}

	newGroup, err := g.GetGroup(group.Name)
	if err != nil {
		return models.Group{}, fmt.Errorf("group '%s': %w", group.Name, err)
	}
	return newGroup, nil
}

func (g *GroupServiceImpl) GetUserGroups(userID string) ([]models.UserGroup, error) {
	var user models.User
	var groups []models.UserGroup
	var userGroups []models.Group
	res := g.db.Where("id = ?", userID).Find(&user)
	if res.Error != nil {
		return []models.UserGroup{}, fmt.Errorf("user '%s': %w", userID, res.Error)
	}

	if res.RowsAffected == 0 {
		return []models.UserGroup{}, fmt.Errorf("user '%s': %w", userID, ErrSvcObjNotFound)
	}

	res = g.db.Model(&models.Group{}).Where("? = ANY(users)", userID).Find(&userGroups)
	if res.Error != nil {
		return []models.UserGroup{}, fmt.Errorf("failed to get user '%s' groups: %w", userID, res.Error)
	}
	for _, grp := range userGroups {
		group, err := g.GetGroupByID(grp.ID.String())
		if err != nil {
			return []models.UserGroup{}, fmt.Errorf("user '%s' groups: %w", userID, err)
		}
		groups = append(groups, models.UserGroup{
			ID:       group.ID,
			Name:     group.Name,
			Provider: group.Provider,
		})
	}

	return groups, nil
}

func (g *GroupServiceImpl) ListGroupUsers(id string) ([]models.GroupUser, error) {
	var users []models.GroupUser

	group, err := g.GetGroupByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	for _, userID := range group.User_IDs {
		user, err := g.UserService.GetUser(userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		users = append(users, models.GroupUser{
			Username: user.Username,
			Provider: user.Provider,
			ID:       user.ID,
		})
	}

	return users, nil
}

func (g *GroupServiceImpl) AddUsersToGroup(id string, users []models.GroupUser) (models.Group, error) {
	group, err := g.GetGroupByID(id)
	if err != nil {
		return models.Group{}, fmt.Errorf("failed to get group: %w", err)
	}

	groupProvider := group.Provider

	for _, u := range users {
		if u.Provider != groupProvider {
			return models.Group{}, fmt.Errorf("group provider '%s', user provider '%s': %w",
				groupProvider,
				u.Provider,
				ErrSvcAuthProviderMismatch,
			)
		}
	}

	var usersID []string

	for _, u := range users {
		user, err := g.UserService.GetByUsernameAndProvider(u.Username, u.Provider)
		if err != nil {
			return models.Group{}, fmt.Errorf("failed to get user: %w", err)
		}
		usersID = append(usersID, user.ID.String())
	}

	group.User_IDs = RemoveDuplicates(append(group.User_IDs, usersID...))

	newGroup, err := g.UpdateGroup(group)
	if err != nil {
		return models.Group{}, fmt.Errorf("update group: %w", err)
	}

	return newGroup, nil
}

func (g *GroupServiceImpl) RemoveUsersFromGroup(id string, users []models.GroupUser) (models.Group, error) {
	group, err := g.GetGroupByID(id)
	if err != nil {
		return models.Group{}, fmt.Errorf("failed to get group: %w", err)
	}

	var usersID []string

	for _, u := range users {
		user, err := g.UserService.GetByUsernameAndProvider(u.Username, u.Provider)
		if err != nil {
			return models.Group{}, fmt.Errorf("failed to get user: %w", err)
		}
		usersID = append(usersID, user.ID.String())
	}

	group.User_IDs = RemoveFromSlice(group.User_IDs, usersID)

	newGroup, err := g.UpdateGroup(group)
	if err != nil {
		return models.Group{}, fmt.Errorf("update groups: %w", err)
	}

	return newGroup, nil
}

func (g *GroupServiceImpl) DeleteGroup(id string) error {
	group, err := g.GetGroupByID(id)
	if err != nil {
		return fmt.Errorf("failed to get group: %w", err)
	}

	if len(group.User_IDs) != 0 {
		return fmt.Errorf("group '%s' in use, remove users first: %w", id, ErrSvcObjInUse)
	}

	res := g.db.Unscoped().Delete(&group)
	if res.Error != nil {
		return fmt.Errorf("group '%s': %w", id, res.Error)
	}
	return nil
}

func (g *GroupServiceImpl) GetGroupRoles(id string) ([]models.Role, error) {
	var roles []models.Role
	var role models.Role

	group, err := g.GetGroupByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	for _, roleID := range group.Role_IDs {
		res := g.db.Model(&models.Role{}).Where("id = ?", roleID).Find(&role)
		if res.Error != nil {
			return []models.Role{}, fmt.Errorf("group '%s' roles: %w", id, res.Error)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func RemoveDuplicates(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func RemoveFromSlice(current []string, input []string) []string {
	for key, value := range current {
		if slices.Contains(input, value) {
			current = append(current[:key], current[key+1:]...)
		}
	}
	return current
}

func (g *GroupServiceImpl) AddRolesToGroup(id string, roles []models.GroupRole) (models.Group, error) {
	var roleIDs []string
	group, err := g.GetGroupByID(id)
	if err != nil {
		return models.Group{}, fmt.Errorf("failed to get group: %w", err)
	}

	for _, r := range roles {
		var role models.Role
		res := g.db.Model(&models.Role{}).Where("name = ?", r.Name).Find(&role)
		if res.Error != nil {
			return models.Group{}, fmt.Errorf("role '%s': %w", r.Name, res.Error)
		}
		if res.RowsAffected == 0 {
			return models.Group{}, fmt.Errorf("role '%s': %w", r.Name, ErrSvcObjNotFound)
		}
		roleIDs = append(roleIDs, role.ID.String())
	}

	group.Role_IDs = RemoveDuplicates(append(group.Role_IDs, roleIDs...))

	newGroup, err := g.UpdateGroup(group)
	if err != nil {
		return models.Group{}, fmt.Errorf("update group: %w", err)
	}

	return newGroup, nil
}

func (g *GroupServiceImpl) RemoveRolesFromGroup(id string, roles []models.GroupRole) (models.Group, error) {
	group, err := g.GetGroupByID(id)
	if err != nil {
		return models.Group{}, fmt.Errorf("failed to get group: %w", err)
	}

	var roleIDs []string

	for _, r := range roles {
		var role models.Role
		res := g.db.Model(&models.Role{}).Where("name = ?", r.Name).Find(&role)
		if res.Error != nil {
			return models.Group{}, fmt.Errorf("role '%s': %w", r.Name, res.Error)
		}
		if res.RowsAffected == 0 {
			return models.Group{}, fmt.Errorf("role '%s': %w", r.Name, ErrSvcObjNotFound)
		}
		roleIDs = append(roleIDs, role.ID.String())
	}

	group.Role_IDs = RemoveFromSlice(group.Role_IDs, roleIDs)

	newGroup, err := g.UpdateGroup(group)
	if err != nil {
		return models.Group{}, fmt.Errorf("update groups: %w", err)
	}

	return newGroup, nil
}

func (g *GroupServiceImpl) ListGroupRoles(id string) ([]models.GroupRole, error) {
	var roles []models.Role
	var groupRoles []models.GroupRole

	group, err := g.GetGroupByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	roles, err = g.GetGroupRoles(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get group %s roles: %w", group.Name, err)
	}
	for _, r := range roles {
		groupRoles = append(groupRoles, models.GroupRole{
			Name:           r.Name,
			Resource:       r.Resource,
			Resource_Names: r.Resource_Names,
			Access:         r.Access,
		})
	}

	return groupRoles, nil
}
