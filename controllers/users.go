package controllers

import (
	"errors"
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
	GroupService  services.GroupService
	AuthService   services.AuthService
	providers     []string
	AuditService  services.AuditService
	AuditCategory string
}

func NewUserController(userService services.UserService, gs services.GroupService, as services.AuthService, als services.AuditService, p []string) UserController {
	return UserController{
		UserService:   userService,
		GroupService:  gs,
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
	r.GET("/:id/groups", middlewares.AuthorizationMiddleware(uc.AuthService, "users", "read"), uc.GetUserGroups)

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
//	@Param			limit	query		int		false	"Maximum number of users to return (default 100)"
//	@Param			offset	query		int		false	"Number of users to skip (default 0)"
//	@Param			name	query		string	false	"Filter by user name"
//	@Success		200	{array}		models.User
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/users [get]
//	@Security		Bearer
func (uc *UserController) ListUsers(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	var params models.UserQueryParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		ctx.Error(errors.New("invalid query parameters"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	users, err := uc.UserService.ListUsers(authList, params)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(users)))
	if len(users) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

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

	_, err := uuid.FromString(userID)
	if err != nil {
		ctx.Error(errors.New("invalid user id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	user, err := uc.UserService.GetUser(userID)

	if err != nil {
		ctx.Error(err)
		return
	}
	user.Groups = []string{}
	ctx.JSON(http.StatusOK, user)
}

// GetUser godoc
//
//	@Summary		Get user groups
//	@Description	Get groups memberships for a user
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{array}		models.UserGroup
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/users/{id} [get]
//	@Security		Bearer
func (uc *UserController) GetUserGroups(ctx *gin.Context) {
	userID := ctx.Param("id")

	_, err := uuid.FromString(userID)
	if err != nil {
		ctx.Error(errors.New("invalid user id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	groups, err := uc.GroupService.GetUserGroups(userID)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, groups)
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
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/users [post]
//	@Security		Bearer
func (uc *UserController) CreateUser(ctx *gin.Context) {
	audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, "*")
	var user models.User

	if err := ctx.ShouldBindJSON(&user); err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid user payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	audit.EventTarget = user.Username

	if !slices.Contains(uc.providers, user.Provider) {
		uc.AuditService.CreateAudit(audit)
		ctx.Error(fmt.Errorf("invalid provider, supported providers: %s", uc.providers))
		ctx.Status(http.StatusBadRequest)
		return
	}

	user, err := uc.UserService.CreateUser(user)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.Error(err)
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
		ctx.Error(errors.New("invalid user payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if !slices.Contains(uc.providers, user.Provider) {
		uc.AuditService.CreateAudit(audit)
		ctx.Error(fmt.Errorf("invalid provider, supported providers: %s", uc.providers))
		ctx.Status(http.StatusBadRequest)
		return
	}

	user.ID, err = uuid.FromString(userID)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid user id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	user, err = uc.UserService.UpdateUser(user)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.Error(err)
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
//	@Success		200	{object}	models.ResponseMessage
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		409	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/users/{id} [delete]
//	@Security		Bearer
func (uc *UserController) DeleteUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, userID)

	_, err := uuid.FromString(userID)
	if err != nil {
		ctx.Error(errors.New("invalid user id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	err = uc.UserService.DeleteUser(userID)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}
	audit.Status = "success"
	uc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "user deleted successfully"})
}
