package models

import (
	"time"

	"github.com/satori/go.uuid"
)

type AuditLog struct {
	ID uuid.UUID `gorm:"column:auditlog_id;type:uuid;default:gen_random_uuid()"`
	// Timestamp time.Time  `json:"@timestamp"`
	User      AuditUser  `json:"user"`
	Event     AuditEvent `json:"event"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type AuditUser struct {
	ID       uuid.UUID
	Name     string `json:"name"`
	Provider string `json:"provider"`
	IP       string `json:"ip"`
}

type AuditEvent struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	Target   string `json:"target"`
	Status   string `json:"status"`
}
