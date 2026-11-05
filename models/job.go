package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type Job struct {
	ID             uuid.UUID              `gorm:"column:id;type:uuid;default:gen_random_uuid()" json:"id"`
	Name           string                 `gorm:"uniqueIndex" json:"name"`
	TaskID         uuid.UUID              `gorm:"type:uuid;index;not null;default:'00000000-0000-0000-0000-000000000000'" json:"task_id"`
	TaskName       string                 `gorm:"index" json:"task_name"`
	Owner          string                 `json:"owner"`
	StartTime      string                 `json:"start_time,omitempty"`
	CompletionTime string                 `json:"completion_time,omitempty"`
	Failed         int32                  `json:"failed"`
	Completed      int32                  `json:"completed"`
	Status         string                 `gorm:"index" json:"status,omitempty"`
	FailReason     string                 `json:"failed_reason,omitempty"`
	Stdout         string                 `json:"stdout,omitempty"`
	JsonData       map[string]interface{} `gorm:"serializer:json;type:jsonb" json:"json_data,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type JobQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Owner  string `form:"owner" binding:"omitempty"`
	Status string `form:"status" binding:"omitempty,oneof=running completed failed"`
	Name   string `form:"name" binding:"omitempty"`
}

type JobMessage struct {
	Message string    `json:"message"`
	JobID   uuid.UUID `json:"id"`
	JobName string    `json:"name"`
}
