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
	}

}

// ListCronJobs godoc
//
//	@Summary		List all Cronjobs
//	@Description	List all Cronjobs
//	@Tags			cronjobs
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		string
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
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"CronJob  id"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{id} [get]
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
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task  name"
//	@Param			evars	body		object	false	"Extra vars"
//	@Success		200		{object}	models.Task
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/jobs/{id} [post]
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

	if job.ID != "" {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, job)
		return
	}

	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "job created successfully", "id": job.ID})
}

// GetSchema godoc
//
//	@Summary		Get task schema
//	@Description	Get task schema for the job info and input parameters
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task  name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{id}/schema [get]
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
