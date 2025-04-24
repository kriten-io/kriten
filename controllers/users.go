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

type UserController struct {
	UserService   services.UserService
	AuthService   services.AuthService
	providers     []string
	AuditService  services.AuditService
	AuditCategory string
}

func NewUserController(userService services.UserService, as services.AuthService, als services.AuditService, p []string) UserController {
	return UserController{
		UserService:   userService,
		AuthService:   as,
		providers:     p,
		AuditService:  als,
		AuditCategory: "users",
	}
}

func (uc *UserController) SetUserRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(uc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(uc.AuthService, "users"), uc.ListUsers)
	r.GET("/:id", middlewares.AuthorizationMiddleware(uc.AuthService, "users", "read"), uc.GetUser)

	r.Use(middlewares.AuthorizationMiddleware(uc.AuthService, "users", "write"))
	{
		r.POST("", uc.CreateUser)
		r.PUT("", uc.CreateUser)
		r.PATCH("/:id", uc.UpdateUser)
		r.PUT("/:id", uc.UpdateUser)
		r.DELETE("/:id", uc.DeleteUser)
	}
}

// ListUsers godoc
//
//	@Summary		List all users
//	@Description	List all users available on the cluster
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.User
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/users [get]
//	@Security		Bearer
func (uc *UserController) ListUsers(ctx *gin.Context) {
	// audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, "*")
	authList := ctx.MustGet("authList").([]string)
	users, err := uc.UserService.ListUsers(authList)

	if err != nil {
		// uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// audit.Status = "success"
	ctx.Header("Content-range", fmt.Sprintf("%v", len(users)))
	if len(users) == 0 {
		var arr [0]int
		// uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	// uc.AuditService.CreateAudit(audit)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, users)
}

// GetUser godoc
//
//	@Summary		Get a user
//	@Description	Get information about a specific user
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	models.User
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/users/{id} [get]
//	@Security		Bearer
func (uc *UserController) GetUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	// audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, userID)
	user, err := uc.UserService.GetUser(userID)

	if err != nil {
		// uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// audit.Status = "success"
	// uc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, user)
}

// CreateUser godoc
//
//	@Summary		Create a new user
//	@Description	Add a user to the cluster
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			user	body		models.User	true	"New user"
//	@Success		200		{object}	models.User
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/users [post]
//	@Security		Bearer
func (uc *UserController) CreateUser(ctx *gin.Context) {
	audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, "*")
	var user models.User

	if err := ctx.ShouldBindJSON(&user); err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit.EventTarget = user.Username

	if !slices.Contains(uc.providers, user.Provider) {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": uc.providers})
		return
	}

	user, err := uc.UserService.CreateUser(user)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	ctx.JSON(http.StatusOK, user)
}

// UpdateUser godoc
//
//	@Summary		Update a user
//	@Description	Update a user in the cluster
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string		true	"User ID"
//	@Param			user	body		models.User	true	"Update user"
//	@Success		200		{object}	models.User
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/users/{id} [patch]
//	@Security		Bearer
func (uc *UserController) UpdateUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, userID)
	var user models.User
	var err error

	if err := ctx.ShouldBindJSON(&user); err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(uc.providers, user.Provider) {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": uc.providers})
		return
	}

	user.ID, err = uuid.FromString(userID)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	user, err = uc.UserService.UpdateUser(user)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	uc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, user)
}

// DeleteUser godoc
//
//	@Summary		Delete a user
//	@Description	Delete by user ID
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"User ID"
//	@Success		204	{object}	models.User
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/users/{id} [delete]
//	@Security		Bearer
func (uc *UserController) DeleteUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, userID)

	err := uc.UserService.DeleteUser(userID)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	uc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "user deleted successfully"})
}
