package models

import (
	"time"

	"github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
)

type Role struct {
	ID           uuid.UUID      `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Name         string         `gorm:"uniqueIndex;<-:create" json:"name" binding:"required"`
	Resource     string         `json:"resource" binding:"required"`
	Resource_IDs pq.StringArray `gorm:"type:text[]" json:"resource_ids" binding:"required,unique"`
	Access       string         `json:"access" binding:"required"`
	Builtin      bool           `json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type RoleQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
