package controllers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"
	uuid "github.com/satori/go.uuid"

	"github.com/gin-gonic/gin"
)

// TODO: This is currently hardcoded but needs to be fetched from somewhere else
var subjectKinds = []string{"groups"}

type RoleBindingController struct {
	RoleBindingService services.RoleBindingService
	AuthService        services.AuthService
	AuditService       services.AuditService
	AuditCategory      string
	providers          []string
}

func NewRoleBindingController(rbs services.RoleBindingService, as services.AuthService, als services.AuditService, p []string) RoleBindingController {
	return RoleBindingController{
		RoleBindingService: rbs,
		AuthService:        as,
		providers:          p,
		AuditService:       als,
		AuditCategory:      "groups",
	}
}

func (rc *RoleBindingController) SetRoleBindingRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(rc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(rc.AuthService, "role_bindings"), rc.ListRoleBindings)
	r.GET("/:id", middlewares.AuthorizationMiddleware(rc.AuthService, "role_bindings", "read"), rc.GetRoleBinding)

	r.Use(middlewares.AuthorizationMiddleware(rc.AuthService, "role_bindings", "write"))
	{
		r.POST("", rc.CreateRoleBinding)
		r.PUT("", rc.CreateRoleBinding)
		r.PATCH("/:id", rc.UpdateRoleBinding)
		r.PUT("/:id", rc.UpdateRoleBinding)
		r.DELETE("/:id", rc.DeleteRoleBinding)
	}
}

// ListRoleBindings godoc
//
//	@Summary		List all role bindings
//	@Description	List all roles bindings available on the cluster
//	@Tags			rolebindings
//	@Accept			json
//	@Produce		json
//	@Param			limit	query	    int		false	"Maximum number of role bindings to return (default 100)"
//	@Param			offset	query		int		false	"Number of role bindings to skip (default 0)"
//	@Param			name	query		string	false	"Filter by role binding name"
//	@Success		200	{array}		models.RoleBinding
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/role_bindings [get]
//	@Security		Bearer
func (rc *RoleBindingController) ListRoleBindings(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	var params models.RoleBindingQueryParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		ctx.Error(errors.New("invalid query parameters"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	roles, err := rc.RoleBindingService.ListRoleBindings(authList, params)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(roles)))
	if len(roles) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, roles)
}

// GetRoleBinding godoc
//
//	@Summary		Get a role binding
//	@Description	Get information about a specific role binding
//	@Tags			rolebindings
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"RoleBinding ID"
//	@Success		200	{object}	models.RoleBinding
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/role_bindings/{id} [get]
//	@Security		Bearer
func (rc *RoleBindingController) GetRoleBinding(ctx *gin.Context) {
	roleBindingID := ctx.Param("id")

	_, err := uuid.FromString(roleBindingID)
	if err != nil {
		ctx.Error(errors.New("invalid role binding id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	roleBinding, err := rc.RoleBindingService.GetRoleBinding(roleBindingID)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, roleBinding)
}

// CreateRoleBinding godoc
//
//	@Summary		Create a new role binding
//	@Description	Add a role binding to the cluster
//	@Tags			rolebindings
//	@Accept			json
//	@Produce		json
//	@Param			roleBinding	body		models.RoleBinding	true	"New role binding"
//	@Success		200			{object}	models.RoleBinding
//	@Failure		400			{object}	helpers.HTTPError
//	@Failure		500			{object}	helpers.HTTPError
//	@Router			/role_bindings [post]
//	@Security		Bearer
func (rc *RoleBindingController) CreateRoleBinding(ctx *gin.Context) {
	audit := rc.AuditService.InitialiseAuditLog(ctx, "create", rc.AuditCategory, "*")
	var roleBinding models.RoleBinding

	if err := ctx.ShouldBindJSON(&roleBinding); err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid role binding payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	audit.EventTarget = roleBinding.Name

	rolebinding, err := rc.RoleBindingService.CreateRoleBinding(roleBinding)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}

	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, rolebinding)
}

// UpdateRoleBinding godoc
//
//	@Summary		Update a role binding
//	@Description	Update a role binding in the cluster
//	@Tags			rolebindings
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"RoleBinding ID"
//	@Param			role	body		models.RoleBinding	true	"Update role"
//	@Success		200		{object}	models.RoleBinding
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/role_bindings/{id} [patch]
//	@Security		Bearer
func (rc *RoleBindingController) UpdateRoleBinding(ctx *gin.Context) {
	roleBindingID := ctx.Param("id")
	audit := rc.AuditService.InitialiseAuditLog(ctx, "update", rc.AuditCategory, roleBindingID)
	var roleBinding models.RoleBinding
	var err error

	if err := ctx.ShouldBindJSON(&roleBinding); err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid role binding payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	roleBinding.ID, err = uuid.FromString(roleBindingID)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		helpers.BadRequestError(ctx, err)
		return
	}

	roleBinding, err = rc.RoleBindingService.UpdateRoleBinding(roleBinding)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		if err != nil {
			ctx.Error(err)
			return
		}
	}
	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, roleBinding)
}

// DeleteRoleBinding godoc
//
//	@Summary		Delete a role binding
//	@Description	Delete by role binding ID
//	@Tags			rolebindings
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"RoleBinding ID"
//	@Success		200	{object}	models.ResponseMessage
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/role_bindings/{id} [delete]
//	@Security		Bearer
func (rc *RoleBindingController) DeleteRoleBinding(ctx *gin.Context) {
	roleBindingID := ctx.Param("id")
	audit := rc.AuditService.InitialiseAuditLog(ctx, "delete", rc.AuditCategory, roleBindingID)

	err := rc.RoleBindingService.DeleteRoleBinding(roleBindingID)
	if err != nil {
		ctx.Error(err)
		return
	}
	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "role binding deleted successfully"})
}
