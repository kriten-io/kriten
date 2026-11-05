package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type Runner struct {
	ID             uuid.UUID         `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Name           string            `gorm:"uniqueIndex;<-:create" json:"name" binding:"required"`
	Image          string            `json:"image" binding:"required"`
	GitURL         string            `json:"gitURL" binding:"omitempty"`
	Branch         string            `json:"branch"`
	TokenEncrypted string            `gorm:"column:token_encrypted" json:"-"`
	Token          string            `gorm:"-" json:"token,omitempty"`
	Secret         map[string]string `gorm:"-" json:"secret,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type RunnerSecret struct {
	ID             uuid.UUID `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	RunnerID       uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_runner_secret_key" json:"runner_id"`
	Key            string    `gorm:"uniqueIndex:idx_runner_secret_key" json:"key"`
	ValueEncrypted string    `gorm:"column:value_encrypted" json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type RunnerQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
