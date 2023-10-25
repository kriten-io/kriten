package models

import (
	"time"

	"github.com/satori/go.uuid"
)

type User struct {
	ID        uuid.UUID `gorm:"column:user_id;type:uuid;default:gen_random_uuid()" json:"user_id"`
	Username  string    `gorm:"uniqueIndex:idx_user,priority:2;<-:create" json:"username" binding:"required"`
	Password  string    `json:"password" binding:"required"`
	Provider  string    `gorm:"uniqueIndex:idx_user,priority:1" json:"provider" binding:"required"`
	Buitin    bool      `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
