package models

import (
	"time"

	"github.com/satori/go.uuid"
)

type ApiToken struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Key         string    `json:"key"`
	Owner       uuid.UUID `gorm:"type:uuid" json:"owner"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Expires     time.Time `json:"expires"`
	Enabled     *bool     `json:"enabled"`
}
