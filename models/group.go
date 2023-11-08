package models

import (
	"time"

	"github.com/satori/go.uuid"
)

type Group struct {
	ID        uuid.UUID `gorm:"column:group_id;type:uuid;default:gen_random_uuid()" json:"group_id"`
	Name      string    `gorm:"uniqueIndex:idx_group,priority:2;<-:create" json:"name" binding:"required"`
	Provider  string    `gorm:"uniqueIndex:idx_group,priority:1" json:"provider" binding:"required"`
	Users     []User    `gorm:"column:users;type:users" json:"users"`
	Buitin    bool      `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
