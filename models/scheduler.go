package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type Scheduler struct {
	ID          uuid.UUID      `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"uniqueIndex;<-:create" json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	TaskID      uuid.UUID      `gorm:"type:uuid;index;not null;default:'00000000-0000-0000-0000-000000000000'" json:"task_id" binding:"required"`
	TaskName    string         `gorm:"index" json:"task_name"`
	CronExpr    string         `json:"cron_expr" binding:"required"`
	ExtraVars   map[string]any `gorm:"serializer:json;type:jsonb" json:"extra_vars,omitempty"`
	Enabled     bool           `json:"enabled"`
	NextRunAt   *time.Time     `json:"next_run_at,omitempty"`
	LastRunAt   *time.Time     `json:"last_run_at,omitempty"`
	LastError   string         `json:"last_error,omitempty"`
	Owner       uuid.UUID      `gorm:"type:uuid;index" json:"owner"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type SchedulerQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
