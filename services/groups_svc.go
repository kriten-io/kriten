package services

import (
	"fmt"
	"log"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type GroupService interface {
	ListGroups([]string) ([]models.Group, error)
	GetGroup(string) (models.Group, error)
	GetUserGroups(string) ([]models.UserGroup, error)
	GetGroupByID(string) (models.Group, error)
	CreateGroup(models.Group) (models.Group, error)
	UpdateGroup(models.Group) (models.Group, error)
	ListUsersInGroup(string) ([]models.GroupUser, error)
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

func (g *GroupServiceImpl) ListGroups(authList []string) ([]models.Group, error) {
	var groups []models.Group
	var res *gorm.DB

	log.Println(authList)

	if len(authList) == 0 {
		return groups, nil
	}

	if slices.Contains(authList, "*") {
		res = g.db.Find(&groups)
	} else {
		res = g.db.Find(&groups, authList)
	}
	if res.Error != nil {
		return groups, res.Error
	}

	return groups, nil
}

func (g *GroupServiceImpl) GetGroup(name string) (models.Group, error) {
	var group models.Group
	res := g.db.Where("name = ?", name).Find(&group)
	if res.Error != nil {
		return models.Group{}, res.Error
	}

	if group.Name == "" {
		return models.Group{}, fmt.Errorf("group %s not found, please check name", name)
	}

	return group, nil
}

func (g *GroupServiceImpl) GetGroupByID(id string) (models.Group, error) {
	var group models.Group
	res := g.db.Where("group_id = ?", id).Find(&group)
	if res.Error != nil {
		return models.Group{}, res.Error
	}
	if group.Name == "" {
		return models.Group{}, fmt.Errorf("group %s not found, please check id", id)
	}
	return group, nil
}

func (g *GroupServiceImpl) CreateGroup(group models.Group) (models.Group, error) {
	res := g.db.Create(&group)

	return group, res.Error
}

func (g *GroupServiceImpl) UpdateGroup(group models.Group) (models.Group, error) {
	res := g.db.Updates(group)
	if res.Error != nil {
		return models.Group{}, res.Error
	}

	newGroup, err := g.GetGroup(group.Name)
	if err != nil {
		return models.Group{}, err
	}
	return newGroup, nil
}

func (g *GroupServiceImpl) GetUserGroups(id string) ([]models.UserGroup, error) {
	var user models.User
	var groups []models.UserGroup
	res := g.db.Where("user_id = ?", id).Find(&user)
	if res.Error != nil {
		return []models.UserGroup{}, res.Error
	}

	if user.Username == "" {
		return []models.UserGroup{}, fmt.Errorf("user %s not found, please check uuid", id)
	}

	for _, groupID := range user.Groups {
		group, err := g.GetGroupByID(groupID)
		if err != nil {
			return []models.UserGroup{}, err
		}
		groups = append(groups, models.UserGroup{
			ID:       group.ID,
			Name:     group.Name,
			Provider: group.Provider,
		})
	}

	return groups, nil
}

func (g *GroupServiceImpl) ListUsersInGroup(id string) ([]models.GroupUser, error) {
	var users []models.GroupUser

	group, err := g.GetGroupByID(id)
	if err != nil {
		return nil, err
	}

	for _, userID := range group.Users {
		user, err := g.UserService.GetUser(userID)
		if err != nil {
			return nil, err
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
		return models.Group{}, err
	}

	usersID, err := g.UpdateUsers(users, group.ID.String(), "add")
	if err != nil {
		return models.Group{}, err
	}

	group.Users = RemoveDuplicates(append(group.Users, usersID...))

	newGroup, err := g.UpdateGroup(group)
	if err != nil {
		return models.Group{}, err
	}

	return newGroup, nil
}

func (g *GroupServiceImpl) RemoveUsersFromGroup(id string, users []models.GroupUser) (models.Group, error) {
	group, err := g.GetGroupByID(id)
	if err != nil {
		return models.Group{}, err
	}

	usersID, err := g.UpdateUsers(users, group.ID.String(), "remove")
	if err != nil {
		return models.Group{}, err
	}

	group.Users = RemoveFromSlice(group.Users, usersID)

	newGroup, err := g.UpdateGroup(group)
	if err != nil {
		return models.Group{}, err
	}

	return newGroup, nil
}

func (g *GroupServiceImpl) DeleteGroup(id string) error {
	group, err := g.GetGroupByID(id)
	if err != nil {
		return err
	}

	if group.Users != nil {
		return fmt.Errorf("cannot delete group %s, please remove users first", group.Name)
	}

	return g.db.Unscoped().Delete(&group).Error
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
		return []models.Role{}, res.Error
	}

	return roles, nil
}

func (g *GroupServiceImpl) UpdateUsers(users []models.GroupUser, groupID string, operation string) ([]string, error) {
	var usersID []string

	for _, u := range users {
		user, err := g.UserService.GetByUsernameAndProvider(u.Username, u.Provider)
		if err != nil {
			return nil, err
		}
		usersID = append(usersID, user.ID.String())

		if operation == "add" {
			_, err = g.UserService.AddGroup(user, groupID)
		} else {
			_, err = g.UserService.RemoveGroup(user, groupID)
		}
		if err != nil {
			return nil, err
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
