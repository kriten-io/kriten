package controllers

import (
	"errors"
	"net/http"

	"github.com/kriten-io/kriten/models"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

func getAuditMetadata(ctx *gin.Context) models.AuditMetadata {
	meta := models.AuditMetadata{}

	if uid, ok := ctx.Get("userID"); ok {
		meta.UserID = uid.(uuid.UUID)
	}
	if username, ok := ctx.Get("username"); ok {
		meta.UserName = username.(string)
	}
	if provider, ok := ctx.Get("provider"); ok {
		meta.Provider = provider.(string)
	}

	return meta
}

func parseUUIDParam(ctx *gin.Context, param string, resource string) (uuid.UUID, bool) {
	id, err := uuid.FromString(ctx.Param(param))
	if err != nil {
		ctx.Error(errors.New("invalid " + resource + " id format"))
		ctx.Status(http.StatusBadRequest)
		return uuid.Nil, false
	}

	return id, true
}
