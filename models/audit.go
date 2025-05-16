package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type AuditLog struct {
	ID uuid.UUID `gorm:"column:auditlog_id;type:uuid;default:gen_random_uuid()" json:"id"`
	// Timestamp time.Time  `json:"@timestamp"`
	UserID        uuid.UUID `gorm:"column:user_id" json:"user_id"`
	UserName      string    `gorm:"column:user_name" json:"username"`
	Provider      string    `gorm:"column:provider" json:"provider"`
	EventType     string    `gorm:"column:event_type" json:"event_type"`
	EventCategory string    `gorm:"column:event_category" json:"event_category"`
	EventTarget   string    `gorm:"column:event_target" json:"event_target"`
	Status        string    `gorm:"column:status" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
