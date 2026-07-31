package models

import (
	"time"

	"github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
)

type Group struct {
	Name      string         `gorm:"uniqueIndex:idx_group,priority:2;<-:create" json:"name" binding:"required"`
	Provider  string         `gorm:"uniqueIndex:idx_group,priority:1" json:"provider" binding:"required"`
	Users     pq.StringArray `gorm:"column:users;type:text[]" json:"users"`
	Builtin   bool           `json:"-"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	ID        uuid.UUID      `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
}

type GroupUser struct {
	Username string    `json:"name"`
	Provider string    `json:"provider"`
	ID       uuid.UUID `json:"id,omitempty"`
}

type GroupQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
