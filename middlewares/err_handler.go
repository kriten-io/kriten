package middlewares

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/services"
)

func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		if len(ctx.Errors) > 0 {
			err := ctx.Errors.Last()
			switch {
			case errors.Is(err, services.ErrSvcObjExists):
				helpers.ConflictError(ctx, err.Error())
			case errors.Is(err, services.ErrSvcObjInUse):
				helpers.ConflictError(ctx, err.Error())
			case errors.Is(err, services.ErrSvcUserGroupMembership):
				helpers.ConflictError(ctx, err.Error())
			case errors.Is(err, services.ErrSvcObjNameK8sConfMap):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcTaskSchemaValidation):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcRoleValidation):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcRoleBindingValidation):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcDBDuplicatedKey):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcCronJobPrecheck):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcDeleteBuiltin):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcObjNotFound):
				helpers.NotFoundError(ctx, err.Error())
			case errors.Is(err, services.ErrSvcTaskNoRunner):
				helpers.BadRequestError(ctx, err)
			case errors.Is(err, services.ErrSvcAuthProviderMismatch):
				helpers.BadRequestError(ctx, err)
			case ctx.Writer.Status() == http.StatusBadRequest:
				helpers.BadRequestError(ctx, err)
			case ctx.Writer.Status() == http.StatusUnauthorized:
				helpers.UnauthorizedError(ctx, err)
			default:
				helpers.InternalError(ctx, err)
			}
		}
	}
}
