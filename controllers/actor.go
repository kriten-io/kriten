package controllers

import (
	"github.com/kriten-io/kriten/models"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

func getActor(ctx *gin.Context) models.Actor {
	actor := models.Actor{
		UserID:   uuid.Nil,
		Username: "",
		Provider: "",
	}

	if userID, ok := ctx.Get("userID"); ok {
		actor.UserID = userID.(uuid.UUID)
	}
	if username, ok := ctx.Get("username"); ok {
		actor.Username = username.(string)
	}
	if provider, ok := ctx.Get("provider"); ok {
		actor.Provider = provider.(string)
	}

	return actor
}