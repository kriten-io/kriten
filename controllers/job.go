package controllers

import (
	"fmt"
	"io"
	"kriten/config"
	"kriten/helpers"
	"kriten/middlewares"
	"kriten/services"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type JobController struct {
	JobService    services.JobService
	AuthService   services.AuthService
	ElasticSearch helpers.ElasticSearch
}

func NewJobController(js services.JobService, as services.AuthService, es helpers.ElasticSearch) JobController {
	return JobController{
		JobService:    js,
		AuthService:   as,
		ElasticSearch: es,
	}
}

func (jc *JobController) SetJobRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(jc.AuthService, "jobs"), jc.ListJobs)
	r.GET("/:id", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetJob)
	r.GET("/:id/log", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetJobLog)
	r.GET("/:id/data", middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "read"), jc.GetJobData)

	r.Use(middlewares.AuthorizationMiddleware(jc.AuthService, "jobs", "write"))
	{
		r.POST(":id", jc.CreateJob)
		r.PUT(":id", jc.CreateJob)
	}

}

// ListJobs godoc
//
//	@Summary		List all jobs
//	@Description	List all jobs
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		string
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs [get]
//	@Security		Bearer
func (jc *JobController) ListJobs(ctx *gin.Context) {
	// username := ctx.MustGet("username").(string)
	// taskID := ctx.Param("id")
	authList := ctx.MustGet("authList").([]string)

	jobsList, err := jc.JobService.ListJobs(authList)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
//	@Produce		string
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
	job, err := jc.JobService.GetJob(username, jobName)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log := job.Stdout

	ctx.Data(http.StatusOK, "text/plain", []byte(log))
}

// GetJobLog godoc
//
//	@Summary		Get a job json data
//	@Description	Get a job json data if present
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Job  id"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/jobs/{id}/data [get]
//	@Security		Bearer
func (jc *JobController) GetJobData(ctx *gin.Context) {
	username := ctx.MustGet("username").(string)
	jobName := ctx.Param("id")
	job, err := jc.JobService.GetJobData(username, jobName)

	if err != nil {
		if strings.Contains(err.Error(), "JSON") {
			ctx.JSON(http.StatusOK, gin.H{"json_data": "", "error": err.Error()})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"json_data": job})
}

// CreateJob godoc
//
//	@Summary		Create a new job
//	@Description	Add a job to the cluster
//	@Tags			jobs
//	@Accept			json
//	@Produce		json
//	@Param			evars	body		object	false	"Extra vars"
//	@Success		200		{object}	models.Task
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/jobs [post]
//	@Security		Bearer
func (jc *JobController) CreateJob(ctx *gin.Context) {
	timestamp := time.Now().UTC()
	username := ctx.MustGet("username").(string)
	taskID := ctx.Param("id")

	extraVars, err := io.ReadAll(ctx.Request.Body)

	if err != nil {
		helpers.CreateElasticSearchLog(jc.ElasticSearch, timestamp, username, ctx.ClientIP(), "launch", "jobs", taskID, "failure")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	jobID, sync, err := jc.JobService.CreateJob(username, taskID, string(extraVars))

	if err != nil {
		helpers.CreateElasticSearchLog(jc.ElasticSearch, timestamp, username, ctx.ClientIP(), "launch", "jobs", taskID, "failure")
		ctx.JSON(http.StatusInternalServerError, gin.H{"Error": err.Error()})
		return
	}

	helpers.CreateElasticSearchLog(jc.ElasticSearch, timestamp, username, ctx.ClientIP(), "launch", "jobs", jobID, "success")
	if sync != "" {
		ctx.JSON(http.StatusOK, gin.H{"logs": sync})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"msg": "job executed successfully", "value": jobID})
}
