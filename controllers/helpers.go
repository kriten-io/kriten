package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

func parseUUIDParam(ctx *gin.Context, param string, resource string) (uuid.UUID, bool) {
	id, err := uuid.FromString(ctx.Param(param))
	if err != nil {
		ctx.Error(errors.New("invalid " + resource + " id format"))
		ctx.Status(http.StatusBadRequest)
		return uuid.Nil, false
	}

	return id, true
}
