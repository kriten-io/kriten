package models

import (
	"time"

	"github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
)

type User struct {
	ID        uuid.UUID      `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Username  string         `gorm:"uniqueIndex:idx_user,priority:2;<-:create" json:"username" binding:"required"`
	Password  string         `json:"password,omitempty" binding:"required"`
	Provider  string         `gorm:"uniqueIndex:idx_user,priority:1" json:"provider" binding:"required"`
	Groups    pq.StringArray `gorm:"column:groups;type:text[]" json:"groups"`
	Builtin   bool           `json:"-"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type UserGroup struct {
	Name     string    `json:"name"`
	Provider string    `json:"provider"`
	ID       uuid.UUID `json:"id"`
}

type UserQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
