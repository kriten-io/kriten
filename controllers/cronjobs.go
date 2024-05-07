package controllers

import (
	"fmt"
	"kriten/config"
	"kriten/middlewares"
	"kriten/models"
	"kriten/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CronJobController struct {
	CronJobService services.CronJobService
	AuthService    services.AuthService
	AuditService   services.AuditService
	AuditCategory  string
}

func NewCronJobController(js services.CronJobService, as services.AuthService, als services.AuditService) CronJobController {
	return CronJobController{
		CronJobService: js,
		AuthService:    as,
		AuditService:   als,
		AuditCategory:  "cronjobs",
	}
}

func (jc *CronJobController) SetCronJobRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(jc.AuthService, "cronjobs"), jc.ListCronJobs)
	r.GET("/:id", middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "read"), jc.GetCronJob)
	r.GET("/:id/schema", middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "read"), jc.GetSchema)

	r.Use(middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "write"))
	{
		r.POST("", jc.CreateCronJob)
		r.PUT("", jc.CreateCronJob)
		r.PATCH("/:id", jc.UpdateCronJob)
		r.PUT("/:id", jc.UpdateCronJob)
		r.DELETE("/:id", jc.DeleteCronJob)
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
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs [get]
//	@Security		Bearer
func (jc *CronJobController) ListCronJobs(ctx *gin.Context) {
	audit := jc.AuditService.InitialiseAuditLog(ctx, "list", jc.AuditCategory, "*")
	authList := ctx.MustGet("authList").([]string)

	jobsList, err := jc.CronJobService.ListCronJobs(authList)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	ctx.Header("Content-range", fmt.Sprintf("%v", len(jobsList)))
	if len(jobsList) == 0 {
		var arr [0]int
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	jc.AuditService.CreateAudit(audit)
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
//	@Param			id	path		string	true	"CronJob  id"
//	@Success		200	{object}	models.CronJob
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs/{id} [get]
//	@Security		Bearer
func (jc *CronJobController) GetCronJob(ctx *gin.Context) {
	jobName := ctx.Param("id")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "get", jc.AuditCategory, jobName)
	job, err := jc.CronJobService.GetCronJob(jobName)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	jc.AuditService.CreateAudit(audit)
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit.EventTarget = cronjob.Task

	cronjob.Owner = username
	job, err := jc.CronJobService.CreateCronJob(cronjob)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
//	@Router			/cronjobs/ [patch]
//	@Security		Bearer
func (jc *CronJobController) UpdateCronJob(ctx *gin.Context) {
	var cronjob models.CronJob
	var err error
	id := ctx.Param("id")
	username := ctx.MustGet("username").(string)
	audit := jc.AuditService.InitialiseAuditLog(ctx, "update", jc.AuditCategory, id)

	if err := ctx.ShouldBindJSON(&cronjob); err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cronjob.Owner = username
	cronjob, err = jc.CronJobService.UpdateCronJob(cronjob)
	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
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
//	@Param			id	path		string	true	"CronJob ID"
//	@Success		204	{object}	models.CronJob
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs/{id} [delete]
//	@Security		Bearer
func (jc *CronJobController) DeleteCronJob(ctx *gin.Context) {
	groupID := ctx.Param("id")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "delete", jc.AuditCategory, groupID)

	err := jc.CronJobService.DeleteCronJob(groupID)
	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "cronjob deleted successfully"})
}

// GetSchema godoc
//
//	@Summary		Get schema
//	@Description	Get schema for the job info and input parameters
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task  name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs/{id}/schema [get]
//	@Security		Bearer
func (jc *CronJobController) GetSchema(ctx *gin.Context) {
	taskName := ctx.Param("id")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "get_schema", jc.AuditCategory, taskName)
	schema, err := jc.CronJobService.GetSchema(taskName)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"

	if schema == nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, gin.H{"msg": "schema not found"})
		return
	}

	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, schema)
}
