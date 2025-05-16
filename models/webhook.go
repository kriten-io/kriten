package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type Webhook struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Owner       uuid.UUID `gorm:"type:uuid" json:"owner"`
	Secret      string    `json:"secret,omitempty"`
	Description string    `json:"description,omitempty"`
	Task        string    `json:"task,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
