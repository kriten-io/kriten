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
	UpdateUsers([]models.GroupUser, string, string) ([]string, error)
	DeleteGroup(string) error
	GetGroupRoles(string, string) ([]models.Role, error)
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

func (g *GroupServiceImpl) GetUserGroups(id string) ([]models.UserGroup, error) {
	var user models.User
	var groups []models.UserGroup
	res := g.db.Where("id = ?", id).Find(&user)
	if res.Error != nil {
		return []models.UserGroup{}, fmt.Errorf("user '%s': %w", id, res.Error)
	}

	if res.RowsAffected == 0 {
		return []models.UserGroup{}, fmt.Errorf("user '%s': %w", id, ErrSvcObjNotFound)
	}

	for _, groupID := range user.Groups {
		group, err := g.GetGroupByID(groupID)
		if err != nil {
			return []models.UserGroup{}, fmt.Errorf("user '%s' groups: %w", id, err)
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

	for _, userID := range group.Users {
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

	usersID, err := g.UpdateUsers(users, group.ID.String(), "add")
	if err != nil {
		return models.Group{}, fmt.Errorf("update users: %w", err)
	}

	group.Users = RemoveDuplicates(append(group.Users, usersID...))

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

	usersID, err := g.UpdateUsers(users, group.ID.String(), "remove")
	if err != nil {
		return models.Group{}, fmt.Errorf("update users: %w", err)
	}

	group.Users = RemoveFromSlice(group.Users, usersID)

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

	if group.Users != nil {
		return fmt.Errorf("group '%s' in use, remove users first: %w", id, ErrSvcObjInUse)
	}

	res := g.db.Unscoped().Delete(&group)
	if res.Error != nil {
		return fmt.Errorf("group '%s': %w", id, res.Error)
	}
	return nil
}

func (g *GroupServiceImpl) GetGroupRoles(subjectID string, provider string) ([]models.Role, error) {
	var roles []models.Role

	// SELECT *
	// FROM roles
	// INNER JOIN role_bindings
	// ON roles.role_id = role_bindings.role_id
	// WHERE role_bindings.subject_provider = provider AND role_bindings.subject_id = subjectID;
	res := g.db.Model(&models.Role{}).Joins(
		"left join role_bindings on roles.role_id = role_bindings.role_id").Where(
		"role_bindings.subject_provider = ? AND role_bindings.subject_id = ?", provider, subjectID).Find(&roles)
	if res.Error != nil {
		return []models.Role{}, fmt.Errorf("group '%s': %w", subjectID, res.Error)
	}

	return roles, nil
}

func (g *GroupServiceImpl) UpdateUsers(users []models.GroupUser, groupID string, operation string) ([]string, error) {
	var usersID []string

	for _, u := range users {
		user, err := g.UserService.GetByUsernameAndProvider(u.Username, u.Provider)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		usersID = append(usersID, user.ID.String())

		if operation == "add" {
			_, err = g.UserService.AddGroup(user, groupID)
		} else {
			_, err = g.UserService.RemoveGroup(user, groupID)
		}
		if err != nil {
			return nil, fmt.Errorf("error updating users: %w", err)
		}
	}

	return usersID, nil
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

func RemoveFromSlice(groupUsers []string, users []string) []string {
	for key, value := range groupUsers {
		if slices.Contains(users, value) {
			groupUsers = append(groupUsers[:key], groupUsers[key+1:]...)
		}
	}
	return groupUsers
}
