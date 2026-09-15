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

type GroupController struct {
	GroupService  services.GroupService
	AuthService   services.AuthService
	AuditService  services.AuditService
	AuditCategory string
	providers     []string
}

func NewGroupController(groupService services.GroupService,
	as services.AuthService,
	als services.AuditService, p []string) GroupController {
	return GroupController{
		GroupService:  groupService,
		AuthService:   as,
		providers:     p,
		AuditService:  als,
		AuditCategory: "groups",
	}
}

func (gc *GroupController) SetGroupRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(gc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(gc.AuthService, "groups"), gc.ListGroups)
	r.GET("/:id", middlewares.AuthorizationMiddleware(gc.AuthService, "groups", "read"), gc.GetGroup)
	r.GET("/:id/users", middlewares.AuthorizationMiddleware(gc.AuthService, "groups", "read"), gc.ListGroupUsers)
	r.GET("/:id/roles", middlewares.AuthorizationMiddleware(gc.AuthService, "groups", "read"), gc.ListGroupRoles)

	r.Use(middlewares.AuthorizationMiddleware(gc.AuthService, "groups", "write"))
	{
		r.POST("", gc.CreateGroup)
		r.PUT("", gc.CreateGroup)
		r.PATCH("/:id", gc.UpdateGroup)
		r.PUT("/:id", gc.UpdateGroup)
		r.DELETE("/:id", gc.DeleteGroup)

		{
			r.POST("/:id/users", gc.AddUsersToGroup)
			r.PUT("/:id/users", gc.AddUsersToGroup)
			r.DELETE("/:id/users", gc.RemoveUsersFromGroup)
		}

		{
			r.POST("/:id/roles", gc.AddRolesToGroup)
			r.PUT("/:id/roles", gc.AddRolesToGroup)
			r.DELETE("/:id/roles", gc.RemoveRolesFromGroup)
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
//	@Param			limit	query		int		false	"Maximum number of groups to return (default 100)"
//	@Param			offset	query		int		false	"Number of groups to skip (default 0)"
//	@Param			name	query		string	false	"Filter by group name"
//	@Success		200	{array}		models.Group
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups [get]
//	@Security		Bearer
func (gc *GroupController) ListGroups(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	var params models.GroupQueryParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		ctx.Error(errors.New("invalid query parameters"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	groups, err := gc.GroupService.ListGroups(authList, params)

	if err != nil {
		ctx.Error(err)
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
func (gc *GroupController) GetGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")

	_, err := uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	group, err := gc.GroupService.GetGroupByID(groupID)

	if err != nil {
		ctx.Error(err)
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
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups [post]
//	@Security		Bearer
func (gc *GroupController) CreateGroup(ctx *gin.Context) {
	audit := gc.AuditService.InitialiseAuditLog(ctx, "create", gc.AuditCategory, "*")
	var group models.Group

	if err := ctx.ShouldBindJSON(&group); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid group payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	audit.EventTarget = group.Name

	if !slices.Contains(gc.providers, group.Provider) {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(fmt.Errorf("provider does not exist, supported providers: %s", gc.providers))
		ctx.Status(http.StatusBadRequest)
		return
	}

	group, err := gc.GroupService.CreateGroup(group)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(err)
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
	groupID := ctx.Param("id")
	audit := gc.AuditService.InitialiseAuditLog(ctx, "update", gc.AuditCategory, groupID)

	_, err = gc.GroupService.GetGroupByID(groupID)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := ctx.ShouldBindJSON(&group); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid group payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if !slices.Contains(gc.providers, group.Provider) {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(fmt.Errorf("provider does not exist, supported providers: %s", gc.providers))
		ctx.Status(http.StatusBadRequest)
		return
	}

	group.ID, err = uuid.FromString(groupID)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
	}

	group, err = gc.GroupService.UpdateGroup(group)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(err)
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
//	@Success		200	{object}	models.ResponseMessage
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups/{id} [delete]
//	@Security		Bearer
func (gc *GroupController) DeleteGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	audit := gc.AuditService.InitialiseAuditLog(ctx, "delete", gc.AuditCategory, groupID)

	_, err := uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	err = gc.GroupService.DeleteGroup(groupID)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}

	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "group deleted successfully"})
}

// ListUsersInGroup godoc
//
//	@Summary		List users
//	@Description	List all users in given group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Group ID"
//	@Success		200	{array}		[]models.GroupUser
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups/{id}/users [get]
//	@Security		Bearer
func (gc *GroupController) ListGroupUsers(ctx *gin.Context) {
	groupID := ctx.Param("id")
	var err error

	_, err = uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	_, err = gc.GroupService.GetGroupByID(groupID)
	if err != nil {
		ctx.Error(err)
		return
	}

	users, err := gc.GroupService.ListGroupUsers(groupID)
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

	ctx.JSON(http.StatusOK, users)
}

// AddUserToGroup godoc
//
//	@Summary		Add users
//	@Description	Add users to group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			group	body		[]models.GroupUser	true	"Users to be added"
//	@Param			id		path		string				true	"Group ID"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups/{id}/users [post]
//	@Security		Bearer
func (gc *GroupController) AddUsersToGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	audit := gc.AuditService.InitialiseAuditLog(ctx, "add_users", gc.AuditCategory, groupID)
	var users []models.GroupUser
	var err error

	_, err = uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	_, err = gc.GroupService.GetGroupByID(groupID)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := ctx.ShouldBindJSON(&users); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid payload format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	group, err := gc.GroupService.AddUsersToGroup(groupID, users)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(err)
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
//	@Param			group	body		[]models.GroupUser	true	"Users to be removed"
//	@Param			id		path		string				true	"Group ID"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups/{id}/users [delete]
//	@Security		Bearer
func (gc *GroupController) RemoveUsersFromGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	audit := gc.AuditService.InitialiseAuditLog(ctx, "remove_users", gc.AuditCategory, groupID)
	var users []models.GroupUser
	var err error

	_, err = uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	_, err = gc.GroupService.GetGroupByID(groupID)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := ctx.ShouldBindJSON(&users); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid payload format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	group, err := gc.GroupService.RemoveUsersFromGroup(groupID, users)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}
	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, group)
}

// AddRolesToGroup godoc
//
//	@Summary		Add roles
//	@Description	Add roles to group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			group	body		[]models.GroupRole	true	"Roles to be added"
//	@Param			id		path		string				true	"Group ID"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups/{id}/roles [post]
//	@Security		Bearer
func (gc *GroupController) AddRolesToGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	audit := gc.AuditService.InitialiseAuditLog(ctx, "add_roles", gc.AuditCategory, groupID)
	var roles []models.GroupRole
	var err error

	_, err = uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	_, err = gc.GroupService.GetGroupByID(groupID)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := ctx.ShouldBindJSON(&roles); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid payload format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	group, err := gc.GroupService.AddRolesToGroup(groupID, roles)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}
	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, group)
}

// RemoveRolesFromGroup godoc
//
//	@Summary		Remove roles
//	@Description	Remove roles from group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			group	body		[]models.GroupRole	true	"Roles to be removed"
//	@Param			id		path		string				true	"Group ID"
//	@Success		200		{object}	models.Group
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/groups/{id}/roles [delete]
//	@Security		Bearer
func (gc *GroupController) RemoveRolesFromGroup(ctx *gin.Context) {
	groupID := ctx.Param("id")
	audit := gc.AuditService.InitialiseAuditLog(ctx, "remove_roles", gc.AuditCategory, groupID)
	var roles []models.GroupRole
	var err error

	_, err = uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	_, err = gc.GroupService.GetGroupByID(groupID)
	if err != nil {
		ctx.Error(err)
		return
	}

	if err := ctx.ShouldBindJSON(&roles); err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid payload format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	group, err := gc.GroupService.RemoveRolesFromGroup(groupID, roles)
	if err != nil {
		gc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}
	audit.Status = "success"
	gc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, group)
}

// ListRolesInGroup godoc
//
//	@Summary		List roles
//	@Description	List all roles in given group
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Group ID"
//	@Success		200	{array}		[]models.GroupRole
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups/{id}/roles [get]
//	@Security		Bearer
func (gc *GroupController) ListGroupRoles(ctx *gin.Context) {
	groupID := ctx.Param("id")
	var err error

	_, err = uuid.FromString(groupID)
	if err != nil {
		ctx.Error(errors.New("invalid group id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	_, err = gc.GroupService.GetGroupByID(groupID)
	if err != nil {
		ctx.Error(err)
		return
	}

	roles, err := gc.GroupService.ListGroupRoles(groupID)
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

	ctx.JSON(http.StatusOK, roles)
}
