package models

import (
	"time"

	"github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
)

type Role struct {
	ID           uuid.UUID      `gorm:"column:role_id;type:uuid;default:gen_random_uuid()" json:"id"`
	Name         string         `gorm:"uniqueIndex;<-:create" json:"name" binding:"required"`
	Resource     string         `json:"resource" binding:"required"`
	Resource_IDs pq.StringArray `gorm:"type:text[]" json:"resource_ids" binding:"required,unique"`
	Access       string         `json:"access" binding:"required"`
	Builtin      bool           `json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}
