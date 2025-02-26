package controllers

import (
	"fmt"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"
	uuid "github.com/satori/go.uuid"

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slices"
)

// TODO: This is currently hardcoded but needs to be fetched from somewhere else
var resources = []string{"runners", "tasks", "jobs", "users", "roles", "role_bindings"}
var access = []string{"read", "write"}

type RoleController struct {
	RoleService   services.RoleService
	AuthService   services.AuthService
	AuditService  services.AuditService
	AuditCategory string
}

func NewRoleController(rs services.RoleService, as services.AuthService, als services.AuditService) RoleController {
	return RoleController{
		RoleService:   rs,
		AuthService:   as,
		AuditService:  als,
		AuditCategory: "roles",
	}
}

func (rc *RoleController) SetRoleRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(rc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(rc.AuthService, "roles"), rc.ListRoles)
	r.GET("/:id", middlewares.AuthorizationMiddleware(rc.AuthService, "roles", "read"), rc.GetRole)

	r.Use(middlewares.AuthorizationMiddleware(rc.AuthService, "roles", "write"))
	{
		r.POST("", rc.CreateRole)
		r.PUT("", rc.CreateRole)
		r.PATCH("/:id", rc.UpdateRole)
		r.PUT("/:id", rc.UpdateRole)
		r.DELETE("/:id", rc.DeleteRole)
	}
}

// ListRoles godoc
//
//	@Summary		List all roles
//	@Description	List all roles available on the cluster
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.Role
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/roles [get]
//	@Security		Bearer
func (rc *RoleController) ListRoles(ctx *gin.Context) {
	//audit := rc.AuditService.InitialiseAuditLog(ctx, "list", rc.AuditCategory, "*")
	authList := ctx.MustGet("authList").([]string)
	roles, err := rc.RoleService.ListRoles(authList)

	if err != nil {
		//rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	//audit.Status = "success"
	ctx.Header("Content-range", fmt.Sprintf("%v", len(roles)))
	if len(roles) == 0 {
		var arr [0]int
		//rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	//rc.AuditService.CreateAudit(audit)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, roles)
}

// GetRole godoc
//
//	@Summary		Get a role
//	@Description	Get information about a specific role
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Role ID"
//	@Success		200	{object}	models.Role
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/roles/{id} [get]
//	@Security		Bearer
func (rc *RoleController) GetRole(ctx *gin.Context) {
	roleID := ctx.Param("id")
	audit := rc.AuditService.InitialiseAuditLog(ctx, "get", rc.AuditCategory, roleID)
	role, err := rc.RoleService.GetRole(roleID)

	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, role)
}

// CreateRole godoc
//
//	@Summary		Create a new role
//	@Description	Add a role to the cluster
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			role	body		models.Role	true	"New role"
//	@Success		200		{object}	models.Role
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/roles [post]
//	@Security		Bearer
func (rc *RoleController) CreateRole(ctx *gin.Context) {
	audit := rc.AuditService.InitialiseAuditLog(ctx, "create", rc.AuditCategory, "*")
	var role models.Role

	if err := ctx.ShouldBindJSON(&role); err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit.EventTarget = role.Name

	if !slices.Contains(resources, role.Resource) {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "resource does not exist", "resources": resources})
		return
	}
	if !slices.Contains(access, role.Access) {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "access not allowed", "access": access})
		return
	}

	role, err := rc.RoleService.CreateRole(role)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, role)
}

// UpdateRole godoc
//
//	@Summary		Update a role
//	@Description	Update a role in the cluster
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string		true	"Role ID"
//	@Param			role	body		models.Role	true	"Update role"
//	@Success		200		{object}	models.Role
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/roles/{id} [patch]
//	@Security		Bearer
func (rc *RoleController) UpdateRole(ctx *gin.Context) {
	roleID := ctx.Param("id")
	audit := rc.AuditService.InitialiseAuditLog(ctx, "update", rc.AuditCategory, roleID)
	var role models.Role
	var err error

	if err := ctx.ShouldBindJSON(&role); err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role.ID, err = uuid.FromString(roleID)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	role, err = rc.RoleService.UpdateRole(role)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, role)
}

// DeleteRole godoc
//
//	@Summary		Delete a role
//	@Description	Delete by role ID
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Role ID"
//	@Success		204	{object}	models.Role
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/roles/{id} [delete]
//	@Security		Bearer
func (rc *RoleController) DeleteRole(ctx *gin.Context) {
	roleID := ctx.Param("id")
	audit := rc.AuditService.InitialiseAuditLog(ctx, "delete", rc.AuditCategory, roleID)

	err := rc.RoleService.DeleteRole(roleID)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "role deleted successfully"})
}
