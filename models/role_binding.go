package models

import (
	"time"

	"github.com/satori/go.uuid"
)

type RoleBinding struct {
	ID              uuid.UUID `gorm:"column:role_binding_id;type:uuid;default:gen_random_uuid()" json:"role_binding_id"`
	Name            string    `gorm:"uniqueIndex;<-:create" json:"name" binding:"required"`
	RoleID          uuid.UUID `gorm:"column:role_id;type:uuid" json:"role_id"`
	RoleName        string    `gorm:"column:role_name" json:"role_name" binding:"required"`
	SubjectKind     string    `json:"subject_kind" binding:"required"`
	SubjectProvider string    `gorm:"index" json:"subject_provider" binding:"required"`
	SubjectID       uuid.UUID `json:"subject_id"`
	SubjectName     string    `gorm:"column:subject_name" json:"subject_name" binding:"required"`
	Buitin          bool      `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
