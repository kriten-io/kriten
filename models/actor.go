package models

import uuid "github.com/satori/go.uuid"

// Actor represents the authenticated user performing an action.
type Actor struct {
	UserID   uuid.UUID
	Username string
	Provider string
}