package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type RoleBinding struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"uniqueIndex;<-:create" json:"name" binding:"required"`
	RoleID    uuid.UUID `gorm:"column:role_id;type:uuid" json:"role_id"`
	GroupID   uuid.UUID `gorm:"column:group_id;type:uuid" json:"group_id"`
	Builtin   bool      `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RoleBindingQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
