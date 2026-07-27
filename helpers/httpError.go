package helpers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewError(ctx *gin.Context, status int, err error) {
	er := HTTPError{
		Code:  status,
		Error: err.Error(),
	}
	ctx.JSON(status, er)
}

type HTTPError struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func NotFoundError(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusNotFound, HTTPError{Code: 404, Error: msg})
}

func InternalError(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusInternalServerError, HTTPError{Code: 500, Error: err.Error()})
}

func ConflictError(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusConflict, HTTPError{Code: 409, Error: msg})
}

func BadRequestError(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusBadRequest, HTTPError{Code: 400, Error: err.Error()})
}

func BadGatewayError(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusBadGateway, HTTPError{Code: 502, Error: err.Error()})
}

func UnauthorizedError(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusUnauthorized, HTTPError{Code: 401, Error: err.Error()})
}
