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

type CronJobController struct {
	CronJobService services.CronJobService
	AuthService    services.AuthService
	AuditService   services.AuditService
	AuditCategory  string
}

func NewCronJobController(
	js services.CronJobService,
	as services.AuthService,
	als services.AuditService,
) CronJobController {
	return CronJobController{
		CronJobService: js,
		AuthService:    as,
		AuditService:   als,
		AuditCategory:  "cronjobs",
	}
}

func (jc *CronJobController) SetCronJobRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(jc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(jc.AuthService, "cronjobs"), jc.ListCronJobs)
	r.GET("/:name", middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "read"), jc.GetCronJob)
	r.GET("/:name/schema", middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "read"), jc.GetSchema)

	r.Use(middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "write"))
	{
		r.POST("", jc.CreateCronJob)
		r.PUT("", jc.CreateCronJob)
		r.PATCH("/:name", jc.UpdateCronJob)
		r.PUT("/:name", jc.UpdateCronJob)
		r.DELETE("/:name", jc.DeleteCronJob)
	}

}

// ListCronJobs godoc
//
//	@Summary		List all Cronjobs
//	@Description	List all Cronjobs
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.CronJob
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs [get]
//	@Security		Bearer
func (jc *CronJobController) ListCronJobs(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	jobsList, err := jc.CronJobService.ListCronJobs(authList)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(jobsList)))
	if len(jobsList) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, jobsList)
}

// GetCronJob godoc
//
//	@Summary		Get job info
//	@Description	Get information about a specific job
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Param			name	path	string	true	"CronJob  name"
//	@Success		200	{object}	models.CronJob
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs/{name} [get]
//	@Security		Bearer
func (jc *CronJobController) GetCronJob(ctx *gin.Context) {
	jobName := ctx.Param("name")
	job, err := jc.CronJobService.GetCronJob(jobName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, job)
}

// CreateCronJob godoc
//
//	@Summary		Create a new job
//	@Description	Add a job to the cluster
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Param			cronjob	body		models.CronJob	true	"New cronjob"
//	@Success		200		{object}	models.CronJob
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/cronjobs [post]
//	@Security		Bearer
func (jc *CronJobController) CreateCronJob(ctx *gin.Context) {
	var cronjob models.CronJob
	audit := jc.AuditService.InitialiseAuditLog(ctx, "create", jc.AuditCategory, "*")
	username := ctx.MustGet("username").(string)

	if err := ctx.ShouldBindJSON(&cronjob); err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid cronjob payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	audit.EventTarget = cronjob.Task

	cronjob.Owner = username
	job, err := jc.CronJobService.CreateCronJob(cronjob)

	if err != nil {
		ctx.Error(err)
		return
	}

	audit.Status = "success"

	if job.Name != "" {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, job)
		return
	}

	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "job created successfully", "id": job.Name})
}

// UpdateCronJob godoc
//
//	@Summary		Update a cronjob
//	@Description	Update a cronjob in the cluster
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Param			cronjob	body		models.CronJob	true	"Update CronJob"
//	@Success		200		{object}	models.CronJob
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/cronjobs/{name} [patch]
//	@Security		Bearer
func (jc *CronJobController) UpdateCronJob(ctx *gin.Context) {
	var cronjob models.CronJob
	var err error
	name := ctx.Param("name")
	username := ctx.MustGet("username").(string)
	audit := jc.AuditService.InitialiseAuditLog(ctx, "update", jc.AuditCategory, name)

	if err := ctx.ShouldBindJSON(&cronjob); err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid cronjob payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	cronjob.Owner = username
	cronjob, err = jc.CronJobService.UpdateCronJob(cronjob)
	if err != nil {
		ctx.Error(err)
		return
	}
	audit.Status = "success"
	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, cronjob)
}

// DeleteCronJob godoc
//
//	@Summary		Delete a CronJob
//	@Description	Delete by CronJob ID
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"CronJob name"
//	@Success		200	{object}	models.ResponseMessage
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs/{name} [delete]
//	@Security		Bearer
func (jc *CronJobController) DeleteCronJob(ctx *gin.Context) {
	name := ctx.Param("name")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "delete", jc.AuditCategory, name)

	err := jc.CronJobService.DeleteCronJob(name)
	if err != nil {
		ctx.Error(err)
		return
	}

	audit.Status = "success"
	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "cronjob deleted successfully"})
}

// GetSchema godoc
//
//	@Summary		Get schema
//	@Description	Get schema for the job info and input parameters
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Param			name	path	string	true	"Task  name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs/{name}/schema [get]
//	@Security		Bearer
func (jc *CronJobController) GetSchema(ctx *gin.Context) {
	taskName := ctx.Param("name")
	schema, err := jc.CronJobService.GetSchema(taskName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, schema)
}
