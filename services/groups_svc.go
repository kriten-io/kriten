package services

import (
	"fmt"
	"kriten/config"
	"kriten/models"

	"golang.org/x/exp/slices"

	"gorm.io/gorm"
)

type GroupService interface {
	ListGroups([]string) ([]models.Group, error)
	GetGroup(string) (models.Group, error)
	CreateGroup(models.Group) (models.Group, error)
	UpdateGroup(models.Group) (models.Group, error)
	AddUsers(string, []string) (models.Group, error)
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

func (g *GroupServiceImpl) AddUsers(groupName string, users []string) (models.Group, error) {
	group, err := g.GetGroup(groupName)
	if err != nil {
		return models.Group{}, err
	}

	// Checking if users uuid are correct
	for _, id := range users {
		user, err := g.UserService.GetUser(id)
		if err != nil {
			return models.Group{}, err
		}

		_, err = g.UserService.AddGroup(user, group.ID.String())
		if err != nil {
			return models.Group{}, err
		}
	}

	group.Users = RemoveDuplicates(append(group.Users, users...))

	newGroup, err := g.UpdateGroup(group)
	if err != nil {
		return models.Group{}, err
	}

	return newGroup, nil
}

func (g *GroupServiceImpl) DeleteGroup(id string) error {
	group, err := g.GetGroup(id)
	if err != nil {
		return err
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
