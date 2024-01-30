package models

import (
	"time"

	"github.com/satori/go.uuid"
)

type AuditLog struct {
	ID uuid.UUID `gorm:"column:auditlog_id;type:uuid;default:gen_random_uuid()"`
	// Timestamp time.Time  `json:"@timestamp"`
	UserID        uuid.UUID `gorm:"column:user_id"`
	UserName      string    `gorm:"column:user_name"`
	Provider      string    `gorm:"column:provider"`
	EventType     string    `gorm:"column:event_type"`
	EventCategory string    `gorm:"column:event_category"`
	EventTarget   string    `gorm:"column:event_target"`
	Status        string    `gorm:"column:status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
