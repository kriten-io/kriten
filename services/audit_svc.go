package services

import (
	"kriten/config"
	"kriten/models"

	"gorm.io/gorm"
)

type AuditService interface {
	ListAudits([]string) ([]models.AuditLog, error)
	GetAudit(string) (models.AuditLog, error)
	CreateAudit(models.AuditLog) (models.AuditLog, error)
}

type AuditServiceImpl struct {
	db     *gorm.DB
	config config.Config
}

func NewAuditService(database *gorm.DB, config config.Config) AuditService {
	return &AuditServiceImpl{
		db:     database,
		config: config,
	}
}

func (a *AuditServiceImpl) ListAudits(authList []string) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	// var res *gorm.DB

	// if len(authList) == 0 {
	// 	return users, nil
	// }

	// if slices.Contains(authList, "*") {
	// 	res = u.db.Find(&users)
	// } else {
	// 	res = u.db.Find(&users, authList)
	// }
	// if res.Error != nil {
	// 	return users, res.Error
	// }

	return logs, nil
}

func (a *AuditServiceImpl) GetAudit(id string) (models.AuditLog, error) {
	var log models.AuditLog
	// res := u.db.Where("user_id = ?", id).Find(&user)
	// if res.Error != nil {
	// 	return models.User{}, res.Error
	// }
	//
	// if user.Username == "" {
	// 	return models.User{}, fmt.Errorf("user %s not found, please check uuid", id)
	// }

	return log, nil
}

func (a *AuditServiceImpl) CreateAudit(log models.AuditLog) (models.AuditLog, error) {
	// if user.Provider == "local" {
	// 	password, err := HashPassword(user.Password)
	// 	if err != nil {
	// 		return models.User{}, err
	// 	}
	// 	user.Password = password
	// }
	//
	// res := u.db.Create(&user)

	// return user, res.Error
	return log, nil
}
