package controllers

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/middlewares"
	"kriten/models"
	"kriten/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/satori/go.uuid"
	"golang.org/x/exp/slices"
)

type GroupController struct {
	GroupService  services.GroupService
	AuthService   services.AuthService
	ElasticSearch helpers.ElasticSearch
	AuditService  services.AuditService
	AuditCategory string
	providers     []string
}

func NewGroupController(groupService services.GroupService, as services.AuthService, es helpers.ElasticSearch, als services.AuditService, p []string) GroupController {
	return GroupController{
		GroupService:  groupService,
		AuthService:   as,
		ElasticSearch: es,
		providers:     p,
		AuditService:  als,
		AuditCategory: "groups",
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

		{
			r.GET("/:id/users", uc.ListUsersInGroup)
			r.POST("/:id/users", uc.AddUserToGroup)
			r.PUT("/:id/users", uc.AddUserToGroup)
			r.DELETE("/:id/users", uc.RemoveUserFromGroup)
		}
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
func (gc *GroupController) ListGroups(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "list", gc.AuditCategory)
	authList := ctx.MustGet("authList").([]string)
	groups, err := gc.GroupService.ListGroups(authList)

	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	ctx.Header("Content-range", fmt.Sprintf("%v", len(groups)))
	if len(groups) == 0 {
		var arr [0]int
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	gc.AuditService.CreateAudit(audit)
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
func (gc *GroupController) GetGroup(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "get", gc.AuditCategory)
	groupID := ctx.Param("id")
	group, err := gc.GroupService.GetGroup(groupID)

	audit.EventTarget = groupID

	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	gc.AuditService.CreateAudit(audit)
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
func (gc *GroupController) CreateGroup(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "create", gc.AuditCategory)
	var group models.Group

	if err := ctx.ShouldBindJSON(&group); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	audit.EventTarget = group.Name

	if !slices.Contains(gc.providers, group.Provider) {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": gc.providers})
		return
	}

	group, err := gc.GroupService.CreateGroup(group)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	gc.AuditService.CreateAudit(audit)
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
func (gc *GroupController) UpdateGroup(ctx *gin.Context) {
	var group models.Group
	var err error
	audit := gc.AuditService.InitialiseAuditLog(ctx, "update", gc.AuditCategory)
	groupID := ctx.Param("id")

	audit.EventTarget = groupID

	if err := ctx.ShouldBindJSON(&group); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !slices.Contains(gc.providers, group.Provider) {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": gc.providers})
		return
	}

	group.ID, err = uuid.FromString(groupID)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	group, err = gc.GroupService.UpdateGroup(group)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
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
func (gc *GroupController) DeleteGroup(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "delete", gc.AuditCategory)
	groupID := ctx.Param("id")

	audit.EventTarget = groupID

	err := gc.GroupService.DeleteGroup(groupID)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "group deleted successfully"})
}

// ListUsersInGroup godoc
//
//	@Summary		List users
//	@Description	List all users in given group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Group ID"
//	@Success		200	{array}		[]models.GroupsUser
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups/{id}/users [get]
//	@Security		Bearer
func (gc *GroupController) ListUsersInGroup(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "list_users", gc.AuditCategory)
	groupName := ctx.Param("id")
	audit.EventTarget = groupName
	var err error

	users, err := gc.GroupService.ListUsersInGroup(groupName)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	ctx.Header("Content-range", fmt.Sprintf("%v", len(users)))
	if len(users) == 0 {
		var arr [0]int
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, users)
}

// AddUserToGroup godoc
//
//	@Summary		Add users
//	@Description	Add users to group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			group	body		[]models.GroupsUser	true	"Users to be added"
//	@Param			id		path		string				true	"Group ID"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups/{id}/users [post]
//	@Security		Bearer
func (gc *GroupController) AddUserToGroup(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "add_users", gc.AuditCategory)
	groupName := ctx.Param("id")
	audit.EventTarget = groupName
	var users []models.GroupsUser
	var err error

	if err := ctx.ShouldBindJSON(&users); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := gc.GroupService.AddUsersToGroup(groupName, users)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, group)
}

// RemoveUserFromGroup godoc
//
//	@Summary		Remove users
//	@Description	Remove users from group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			group	body		[]models.GroupsUser	true	"Users to be removed"
//	@Param			id		path		string				true	"Group ID"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups/{id}/users [delete]
//	@Security		Bearer
func (gc *GroupController) RemoveUserFromGroup(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "remove_users", gc.AuditCategory)
	groupName := ctx.Param("id")
	audit.EventTarget = groupName
	var users []models.GroupsUser
	var err error

	if err := ctx.ShouldBindJSON(&users); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := gc.GroupService.RemoveUsersFromGroup(groupName, users)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, group)
}
