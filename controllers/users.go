package controllers

import (
	"fmt"
	"kriten-core/config"
	"kriten-core/helpers"
	"kriten-core/middlewares"
	"kriten-core/models"
	"kriten-core/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/satori/go.uuid"
	"golang.org/x/exp/slices"
)

type UserController struct {
	UserService   services.UserService
	AuthService   services.AuthService
	ElasticSearch helpers.ElasticSearch
	providers     []string
}

func NewUserController(userService services.UserService, as services.AuthService, es helpers.ElasticSearch, p []string) UserController {
	return UserController{
		UserService:   userService,
		AuthService:   as,
		ElasticSearch: es,
		providers:     p,
	}
}

func (uc *UserController) SetUserRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

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
func (rc *UserController) ListUsers(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)
	users, err := rc.UserService.ListUsers(authList)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
func (rc *UserController) GetUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	user, err := rc.UserService.GetUser(userID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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
	var user models.User

	if err := ctx.ShouldBindJSON(&user); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(uc.providers, user.Provider) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": uc.providers})
		return
	}

	user, err := uc.UserService.CreateUser(user)
	log.Println(user)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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
	var user models.User
	var err error

	if err := ctx.ShouldBindJSON(&user); err != nil {
		log.Println(err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(uc.providers, user.Provider) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": uc.providers})
		return
	}

	user.ID, err = uuid.FromString(userID)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	user, err = uc.UserService.UpdateUser(user)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
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
func (rc *UserController) DeleteUser(ctx *gin.Context) {
	userID := ctx.Param("id")

	err := rc.UserService.DeleteUser(userID)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"msg": "user deleted successfully"})
}
