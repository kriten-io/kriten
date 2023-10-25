package models

import (
	"github.com/golang-jwt/jwt"
	uuid "github.com/satori/go.uuid"
)

type Credentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Provider string `json:"provider" binding:"required"`
}

type Claims struct {
	Username string    `json:"username"`
	UserID   uuid.UUID `json:"user_id"`
	Provider string    `json:"provider"`
	jwt.StandardClaims
}

type Authorization struct {
	Username   string    `json:"username"`
	UserID     uuid.UUID `json:"user_id"`
	Provider   string    `json:"provider"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Access     string    `json:"access"`
}
