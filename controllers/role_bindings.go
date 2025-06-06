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
//	@Success		200	{array}		models.RoleBinding
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/role_bindings [get]
//	@Security		Bearer
func (rc *RoleBindingController) ListRoleBindings(ctx *gin.Context) {
	//audit := rc.AuditService.InitialiseAuditLog(ctx, "list", rc.AuditCategory, "*")
	filters := make(map[string]string)
	authList := ctx.MustGet("authList").([]string)

	urlParams := ctx.Request.URL.Query()

	// urlParams contains a map[string][]string
	// we need to parse it into a map[string]string
	// so we will only take the first value
	for key, value := range urlParams {
		filters[key] = value[0]
	}

	roles, err := rc.RoleBindingService.ListRoleBindings(authList, filters)

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
	audit := rc.AuditService.InitialiseAuditLog(ctx, "get", rc.AuditCategory, roleBindingID)
	role, err := rc.RoleBindingService.GetRoleBinding(roleBindingID)

	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"

	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, role)
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
//	@Failure		404			{object}	helpers.HTTPError
//	@Failure		500			{object}	helpers.HTTPError
//	@Router			/role_bindings [post]
//	@Security		Bearer
func (rc *RoleBindingController) CreateRoleBinding(ctx *gin.Context) {
	audit := rc.AuditService.InitialiseAuditLog(ctx, "create", rc.AuditCategory, "*")
	var roleBinding models.RoleBinding

	if err := ctx.ShouldBindJSON(&roleBinding); err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	audit.EventTarget = roleBinding.Name

	if !slices.Contains(subjectKinds, roleBinding.SubjectKind) {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "subject kind does not exist", "subject_kinds": subjectKinds})
		return
	}
	if !slices.Contains(rc.providers, roleBinding.SubjectProvider) {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": rc.providers})
		return
	}

	rolebinding, err := rc.RoleBindingService.CreateRoleBinding(roleBinding)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(subjectKinds, roleBinding.SubjectKind) {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "subject kind does not exist", "subject_kinds": subjectKinds})
		return
	}
	if !slices.Contains(rc.providers, roleBinding.SubjectProvider) {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": rc.providers})
		return
	}

	roleBinding.ID, err = uuid.FromString(roleBindingID)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	roleBinding, err = rc.RoleBindingService.UpdateRoleBinding(roleBinding)
	if err != nil {
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
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
//	@Success		204	{object}	models.RoleBinding
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
		rc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	rc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "role binding deleted successfully"})
}
