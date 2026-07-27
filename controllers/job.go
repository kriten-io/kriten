package controllers

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
)

type JobController struct {
	JobService    services.JobService
	AuthService   services.AuthService
	AuditService  services.AuditService
	AuditCategory string
}

func NewJobController(js services.JobService, as services.AuthService, als services.AuditService) JobController {
	return JobController{
		JobService:    js,
		AuthService:   as,
		AuditService:  als,
		AuditCategory: "jobs",
	}
}

func (jc *JobController) SetJobRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(jc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(jc.AuthService, "jobs"), jc.ListJobs)
	r.GET("/:name", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetJob)
	r.GET("/:name/log", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetJobLog)
	r.GET("/:name/schema", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetSchema)

	r.Use(middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "write"))
	{
		r.POST(":name", jc.CreateJob)
		r.PUT(":name", jc.CreateJob)
	}

}

// ListJobs godoc
//
//	@Summary		List all jobs
//	@Description	List all jobs with optional filtering and pagination
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			limit	query		int		false	"Maximum number of jobs to return (default 100)"
//	@Param			offset	query		int		false	"Number of jobs to skip (default 0)"
//	@Param			owner	query		string	false	"Filter by job owner"
//	@Param			status	query		string	false	"Filter by status: running, completed, failed"
//	@Param			name	query		string	false	"Filter by task/job name"
//	@Success		200	{array}		models.Job
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs [get]
//	@Security		Bearer
func (jc *JobController) ListJobs(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	var params models.JobQueryParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		ctx.Error(errors.New("invalid query parameters"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	jobsList, total, err := jc.JobService.ListJobs(authList, params)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", total))
	if len(jobsList) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, jobsList)
}

// GetJob godoc
//
//	@Summary		Get job info
//	@Description	Get information about a specific job
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Job Name"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{name} [get]
//	@Security		Bearer
func (jc *JobController) GetJob(ctx *gin.Context) {
	username := ctx.MustGet("username").(string)
	jobName := ctx.Param("name")
	job, err := jc.JobService.GetJob(username, jobName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, job)
}

// GetJobLog godoc
//
//	@Summary		Get a job log
//	@Description	Get a job log as text
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Job  name"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{name}/log [get]
//	@Security		Bearer
func (jc *JobController) GetJobLog(ctx *gin.Context) {
	username := ctx.MustGet("username").(string)
	jobName := ctx.Param("name")
	log, err := jc.JobService.GetLog(username, jobName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Data(http.StatusOK, "text/plain", []byte(log))
}

// CreateJob godoc
//
//	@Summary		Create a new job
//	@Description	Add a job to the cluster
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Task  name"
//	@Param			evars	body		object	false	"Extra vars"
//	@Success		200		{object}	models.JobMessage
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/jobs/{name} [post]
//	@Security		Bearer
func (jc *JobController) CreateJob(ctx *gin.Context) {
	taskName := ctx.Param("name")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "create", jc.AuditCategory, taskName)
	username := ctx.MustGet("username").(string)

	extraVars, err := io.ReadAll(ctx.Request.Body)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid job payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	job, err := jc.JobService.CreateJob(username, taskName, string(extraVars))

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}

	audit.Status = "success"

	if (job.Name != "") && (job.Completed != 0) {
		jc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, job)
		return
	}

	jc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, models.JobMessage{Message: "job created successfully", JobName: job.Name})
}

// GetSchema godoc
//
//	@Summary		Get task schema
//	@Description	Get task schema for the job info and input parameters
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Task  name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/cronjobs/{name}/schema [get]
//	@Security		Bearer
func (jc *JobController) GetSchema(ctx *gin.Context) {
	taskName := ctx.Param("name")
	schema, err := jc.JobService.GetSchema(taskName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, schema)
}
