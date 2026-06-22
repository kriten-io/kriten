package helpers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewError(ctx *gin.Context, status int, err error) {
	er := HTTPError{
		Code:    status,
		Message: err.Error(),
	}
	ctx.JSON(status, er)
}

type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NotFoundError(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusNotFound, HTTPError{Code: 404, Message: msg})
}

func InternalError(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusInternalServerError, HTTPError{Code: 500, Message: err.Error()})
}

func ConflictError(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusConflict, HTTPError{Code: 409, Message: msg})
}

func BadRequestError(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusBadRequest, HTTPError{Code: 400, Message: msg})
}

func BadGatewayError(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusBadGateway, HTTPError{Code: 502, Message: err.Error()})
}
