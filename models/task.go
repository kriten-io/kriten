package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type Task struct {
	ID          uuid.UUID      `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Schema      map[string]any `gorm:"serializer:json;type:jsonb" json:"schema,omitempty"`
	Name        string         `gorm:"uniqueIndex;<-:create" json:"name" binding:"required"`
	Runner      string         `json:"runner" binding:"required"`
	Command     string         `json:"command" binding:"required"`
	Synchronous bool           `json:"synchronous"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type TaskQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
