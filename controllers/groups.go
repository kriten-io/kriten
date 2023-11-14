package controllers

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/middlewares"
	"kriten/models"
	"kriten/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/satori/go.uuid"
	"golang.org/x/exp/slices"
)

type GroupController struct {
	GroupService  services.GroupService
	AuthService   services.AuthService
	ElasticSearch helpers.ElasticSearch
	providers     []string
}

func NewGroupController(groupService services.GroupService, as services.AuthService, es helpers.ElasticSearch, p []string) GroupController {
	return GroupController{
		GroupService:  groupService,
		AuthService:   as,
		ElasticSearch: es,
		providers:     p,
	}
}

func (uc *GroupController) SetGroupRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(uc.AuthService, "groups"), uc.ListGroups)
	r.GET("/:id", middlewares.AuthorizationMiddleware(uc.AuthService, "groups", "read"), uc.GetGroup)

	r.Use(middlewares.AuthorizationMiddleware(uc.AuthService, "groups", "write"))
	{
		r.POST("", uc.CreateGroup)
		r.PUT("", uc.CreateGroup)
		r.PATCH("/:id", uc.UpdateGroup)
		r.PUT("/:id", uc.UpdateGroup)
		r.DELETE("/:id", uc.DeleteGroup)

		r.PATCH("/:id/add-users", uc.AddUserToGroup)
		r.PATCH("/:id/remove-user", uc.RemoveUserFromGroup)
	}
}

// ListGroups godoc
//
//	@Summary		List all groups
//	@Description	List all groups available on the cluster
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.Group
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups [get]
//	@Security		Bearer
func (rc *GroupController) ListGroups(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)
	groups, err := rc.GroupService.ListGroups(authList)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(groups)))
	if len(groups) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, groups)
}

// GetGroup godoc
//
//	@Summary		Get a group
//	@Description	Get information about a specific group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Group ID"
//	@Success		200	{object}	models.Group
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups/{id} [get]
//	@Security		Bearer
func (rc *GroupController) GetGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	group, err := rc.GroupService.GetGroup(groupID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, group)
}

// CreateGroup godoc
//
//	@Summary		Create a new group
//	@Description	Add a group to the cluster
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			group	body		models.Group	true	"New group"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups [post]
//	@Security		Bearer
func (uc *GroupController) CreateGroup(ctx *gin.Context) {
	var group models.Group

	if err := ctx.ShouldBindJSON(&group); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(uc.providers, group.Provider) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": uc.providers})
		return
	}

	group, err := uc.GroupService.CreateGroup(group)
	log.Println(group)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, group)
}

// UpdateGroup godoc
//
//	@Summary		Update a group
//	@Description	Update a group in the cluster
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string		true	"Group ID"
//	@Param			group	body		models.Group	true	"Update group"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups/{id} [patch]
//	@Security		Bearer
func (uc *GroupController) UpdateGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	var group models.Group
	var err error

	if err := ctx.ShouldBindJSON(&group); err != nil {
		log.Println(err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(uc.providers, group.Provider) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": uc.providers})
		return
	}

	group.ID, err = uuid.FromString(groupID)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	group, err = uc.GroupService.UpdateGroup(group)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, group)
}

// DeleteGroup godoc
//
//	@Summary		Delete a group
//	@Description	Delete by group ID
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Group ID"
//	@Success		204	{object}	models.Group
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups/{id} [delete]
//	@Security		Bearer
func (rc *GroupController) DeleteGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")

	err := rc.GroupService.DeleteGroup(groupID)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"msg": "group deleted successfully"})
}

func (uc *GroupController) AddUserToGroup(ctx *gin.Context) {
	groupName := ctx.Param("id")
	var users []string
	var err error

	if err := ctx.ShouldBindJSON(&users); err != nil {
		log.Println(err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := uc.GroupService.AddUsers(groupName, users)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, group)
}

func (uc *GroupController) RemoveUserFromGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	var group models.Group
	var err error

	if err := ctx.ShouldBindJSON(&group); err != nil {
		log.Println(err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(uc.providers, group.Provider) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": uc.providers})
		return
	}

	group.ID, err = uuid.FromString(groupID)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	group, err = uc.GroupService.UpdateGroup(group)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, group)
}
