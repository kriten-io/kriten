package controllers

import (
	"fmt"
	"io"
	"kriten/config"
	"kriten/middlewares"
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
	r.GET("/:id/log", middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "read"), jc.GetCronJobLog)
	r.GET("/:id/schema", middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "read"), jc.GetSchema)

	r.Use(middlewares.AuthorizationMiddleware(jc.AuthService, "cronjobs", "write"))
	{
		r.POST(":id", jc.CreateCronJob)
		r.PUT(":id", jc.CreateCronJob)
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
	username := ctx.MustGet("username").(string)
	jobName := ctx.Param("id")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "get", jc.AuditCategory, jobName)
	job, err := jc.CronJobService.GetCronJob(username, jobName)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, job)
}

// GetCronJobLog godoc
//
//	@Summary		Get a job log
//	@Description	Get a job log as text
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"CronJob  id"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{id}/log [get]
//	@Security		Bearer
func (jc *CronJobController) GetCronJobLog(ctx *gin.Context) {
	username := ctx.MustGet("username").(string)
	jobName := ctx.Param("id")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "get_job_log", jc.AuditCategory, jobName)
	job, err := jc.CronJobService.GetCronJob(username, jobName)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	log := job.Stdout

	jc.AuditService.CreateAudit(audit)
	ctx.Data(http.StatusOK, "text/plain", []byte(log))
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
	taskID := ctx.Param("id")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "create", jc.AuditCategory, taskID)
	username := ctx.MustGet("username").(string)

	extraVars, err := io.ReadAll(ctx.Request.Body)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	job, err := jc.CronJobService.CreateCronJob(username, taskID, string(extraVars))

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	if (job.ID != "") && (job.Completed != 0) {
		//ctx.JSON(http.StatusOK, gin.H{"id": jobID, "json_data": sync.JsonData})
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
