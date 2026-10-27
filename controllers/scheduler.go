package controllers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
)

type SchedulerController struct {
	SchedulerService services.SchedulerService
	AuthService      services.AuthService
}

func NewSchedulerController(ss services.SchedulerService, as services.AuthService) SchedulerController {
	return SchedulerController{
		SchedulerService: ss,
		AuthService:      as,
	}
}

// Access to individual schedules is decided in the service: the owner always
// has access, others need the "schedulers" resource with the matching access.
func (sc *SchedulerController) SetSchedulerRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(sc.AuthService, config.JWT))

	r.GET("", sc.ListSchedulers)
	r.GET("/:id", sc.GetScheduler)
	r.POST("", sc.CreateScheduler)
	r.PUT("/:id", sc.UpdateScheduler)
	r.DELETE("/:id", sc.DeleteScheduler)
	r.POST("/:id/enable", sc.EnableScheduler)
	r.POST("/:id/disable", sc.DisableScheduler)
}

// ListSchedulers godoc
//
//	@Summary		List schedulers
//	@Description	List own schedulers, or all schedulers for users with schedulers read access
//	@Tags			schedulers
//	@Accept			json
//	@Produce		json
//	@Param			limit	query		int		false	"Maximum number of schedulers to return (default 100)"
//	@Param			offset	query		int		false	"Number of schedulers to skip (default 0)"
//	@Param			name	query		string	false	"Filter by name (case insensitive substring)"
//	@Success		200		{array}		models.Scheduler
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/schedulers [get]
//	@Security		Bearer
func (sc *SchedulerController) ListSchedulers(ctx *gin.Context) {
	var params models.SchedulerQueryParams

	if err := ctx.ShouldBindQuery(&params); err != nil {
		ctx.Error(errors.New("invalid query parameters"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	schedulers, total, err := sc.SchedulerService.ListSchedulers(getAuditMetadata(ctx), params)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", total))
	if len(schedulers) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.JSON(http.StatusOK, schedulers)
}

// GetScheduler godoc
//
//	@Summary		Get a scheduler
//	@Description	Get information about a specific scheduler
//	@Tags			schedulers
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Scheduler ID"
//	@Success		200	{object}	models.Scheduler
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		403	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/schedulers/{id} [get]
//	@Security		Bearer
func (sc *SchedulerController) GetScheduler(ctx *gin.Context) {
	id, ok := parseUUIDParam(ctx, "id", "scheduler")
	if !ok {
		return
	}

	scheduler, err := sc.SchedulerService.GetScheduler(getAuditMetadata(ctx), id.String())
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, scheduler)
}

// CreateScheduler godoc
//
//	@Summary		Create a scheduler
//	@Description	Schedule a task with a cron expression. Requires execute access on the task. Extra vars are validated against the task schema.
//	@Tags			schedulers
//	@Accept			json
//	@Produce		json
//	@Param			scheduler	body		models.Scheduler	true	"New scheduler"
//	@Success		200			{object}	models.Scheduler
//	@Failure		400			{object}	helpers.HTTPError
//	@Failure		403			{object}	helpers.HTTPError
//	@Failure		404			{object}	helpers.HTTPError
//	@Failure		409			{object}	helpers.HTTPError
//	@Failure		500			{object}	helpers.HTTPError
//	@Router			/schedulers [post]
//	@Security		Bearer
func (sc *SchedulerController) CreateScheduler(ctx *gin.Context) {
	var scheduler models.Scheduler

	if err := ctx.ShouldBindJSON(&scheduler); err != nil {
		ctx.Error(errors.New("invalid scheduler payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	created, err := sc.SchedulerService.CreateScheduler(getAuditMetadata(ctx), scheduler)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, created)
}

// UpdateScheduler godoc
//
//	@Summary		Update a scheduler
//	@Description	Update a scheduler. Name is immutable. Re-validates the cron expression and extra vars.
//	@Tags			schedulers
//	@Accept			json
//	@Produce		json
//	@Param			id			path		string				true	"Scheduler ID"
//	@Param			scheduler	body		models.Scheduler	true	"Updated scheduler"
//	@Success		200			{object}	models.Scheduler
//	@Failure		400			{object}	helpers.HTTPError
//	@Failure		403			{object}	helpers.HTTPError
//	@Failure		404			{object}	helpers.HTTPError
//	@Failure		500			{object}	helpers.HTTPError
//	@Router			/schedulers/{id} [put]
//	@Security		Bearer
func (sc *SchedulerController) UpdateScheduler(ctx *gin.Context) {
	id, ok := parseUUIDParam(ctx, "id", "scheduler")
	if !ok {
		return
	}

	var scheduler models.Scheduler
	if err := ctx.ShouldBindJSON(&scheduler); err != nil {
		ctx.Error(errors.New("invalid scheduler payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	updated, err := sc.SchedulerService.UpdateScheduler(getAuditMetadata(ctx), id.String(), scheduler)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, updated)
}

// DeleteScheduler godoc
//
//	@Summary		Delete a scheduler
//	@Description	Delete by scheduler ID
//	@Tags			schedulers
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Scheduler ID"
//	@Success		200	{object}	models.ResponseMessage
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		403	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/schedulers/{id} [delete]
//	@Security		Bearer
func (sc *SchedulerController) DeleteScheduler(ctx *gin.Context) {
	id, ok := parseUUIDParam(ctx, "id", "scheduler")
	if !ok {
		return
	}

	if err := sc.SchedulerService.DeleteScheduler(getAuditMetadata(ctx), id.String()); err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "scheduler deleted successfully"})
}

// EnableScheduler godoc
//
//	@Summary		Enable a scheduler
//	@Description	Enable a scheduler and compute its next run. Re-validates the schedule against the current task schema.
//	@Tags			schedulers
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Scheduler ID"
//	@Success		200	{object}	models.Scheduler
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		403	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/schedulers/{id}/enable [post]
//	@Security		Bearer
func (sc *SchedulerController) EnableScheduler(ctx *gin.Context) {
	id, ok := parseUUIDParam(ctx, "id", "scheduler")
	if !ok {
		return
	}

	scheduler, err := sc.SchedulerService.EnableScheduler(getAuditMetadata(ctx), id.String())
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, scheduler)
}

// DisableScheduler godoc
//
//	@Summary		Disable a scheduler
//	@Description	Disable a scheduler so it no longer fires
//	@Tags			schedulers
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Scheduler ID"
//	@Success		200	{object}	models.Scheduler
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		403	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/schedulers/{id}/disable [post]
//	@Security		Bearer
func (sc *SchedulerController) DisableScheduler(ctx *gin.Context) {
	id, ok := parseUUIDParam(ctx, "id", "scheduler")
	if !ok {
		return
	}

	scheduler, err := sc.SchedulerService.DisableScheduler(getAuditMetadata(ctx), id.String())
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, scheduler)
}
