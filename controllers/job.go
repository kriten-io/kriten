package controllers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/errors"
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
	r.GET("/:id", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetJob)
	r.GET("/:id/log", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetJobLog)
	r.GET("/:id/schema", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetSchema)

	r.Use(middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "write"))
	{
		r.POST(":id", jc.CreateJob)
		r.PUT(":id", jc.CreateJob)
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
//	@Param			job_name	query		string	false	"Filter by task/job name"
//	@Success		200	{array}		models.Job
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs [get]
//	@Security		Bearer
func (jc *JobController) ListJobs(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	var params models.JobQueryParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		helpers.BadRequestError(ctx, err.Error())
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	jobsList, total, err := jc.JobService.ListJobs(authList, params)

	if err != nil {
		helpers.InternalError(ctx, err)
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
//	@Param			id	path		string	true	"Job  id"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{id} [get]
//	@Security		Bearer
func (jc *JobController) GetJob(ctx *gin.Context) {
	username := ctx.MustGet("username").(string)
	jobName := ctx.Param("id")
	job, err := jc.JobService.GetJob(username, jobName)

	if err != nil {
		if errors.IsNotFound(err) {
			helpers.NotFoundError(ctx, "job not found")
			return
		}
		helpers.InternalError(ctx, err)
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
//	@Param			id	path		string	true	"Job  id"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{id}/log [get]
//	@Security		Bearer
func (jc *JobController) GetJobLog(ctx *gin.Context) {
	username := ctx.MustGet("username").(string)
	jobName := ctx.Param("id")
	log, err := jc.JobService.GetLog(username, jobName)

	if err != nil {
		if errors.IsNotFound(err) {
			helpers.NotFoundError(ctx, "job not found")
			return
		}
		helpers.InternalError(ctx, err)
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
//	@Param			id	path		string	true	"Task  name"
//	@Param			evars	body		object	false	"Extra vars"
//	@Success		200		{object}	models.Task
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/jobs/{id} [post]
//	@Security		Bearer
func (jc *JobController) CreateJob(ctx *gin.Context) {
	taskID := ctx.Param("id")
	audit := jc.AuditService.InitialiseAuditLog(ctx, "create", jc.AuditCategory, taskID)
	username := ctx.MustGet("username").(string)

	extraVars, err := io.ReadAll(ctx.Request.Body)

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		helpers.BadRequestError(ctx, err.Error())
		return
	}

	job, err := jc.JobService.CreateJob(username, taskID, string(extraVars))

	if err != nil {
		jc.AuditService.CreateAudit(audit)
		if errors.IsNotFound(err) {
			helpers.NotFoundError(ctx, "task not found")
			return
		}
		helpers.InternalError(ctx, err)
		return
	}

	audit.Status = "success"

	if (job.ID != "") && (job.Completed != 0) {
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
func (jc *JobController) GetSchema(ctx *gin.Context) {
	taskName := ctx.Param("id")
	schema, err := jc.JobService.GetSchema(taskName)

	if err != nil {
		if errors.IsNotFound(err) {
			helpers.NotFoundError(ctx, "task not found")
			return
		}
		helpers.InternalError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, schema)
}
