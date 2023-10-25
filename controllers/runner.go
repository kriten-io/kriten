package controllers

import (
	"fmt"
	"kriten-core/config"
	"kriten-core/helpers"
	"kriten-core/middlewares"
	"kriten-core/models"
	"kriten-core/services"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/errors"
)

type RunnerController struct {
	RunnerService services.RunnerService
	AuthService   services.AuthService
	ElasticSearch helpers.ElasticSearch
}

func NewRunnerController(rs services.RunnerService, as services.AuthService, es helpers.ElasticSearch) RunnerController {
	return RunnerController{
		RunnerService: rs,
		AuthService:   as,
		ElasticSearch: es,
	}
}

func (rc *RunnerController) SetRunnerRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(rc.AuthService, "runners"), rc.ListRunners)
	r.GET("/:id", middlewares.AuthorizationMiddleware(rc.AuthService, "runners", "read"), rc.GetRunner)

	r.Use(middlewares.AuthorizationMiddleware(rc.AuthService, "runners", "write"))
	{
		r.POST("", rc.CreateRunner)
		r.PUT("", rc.CreateRunner)
		r.PATCH("/:id", rc.UpdateRunner)
		r.PUT("/:id", rc.UpdateRunner)
		r.DELETE("/:id", rc.DeleteRunner)
	}

}

// ListRunners godoc
//
//	@Summary		List all runners
//	@Description	List all runners available on the cluster
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.Runner
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/runners [get]
//	@Security		Bearer
func (rc *RunnerController) ListRunners(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)
	runnersList, err := rc.RunnerService.ListRunners(authList)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(runnersList)))
	if len(runnersList) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	// ctx.Header("Content-range", fmt.Sprintf("%v", len(tasksList)))
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, runnersList)
}

// GetRunner godoc
//
//	@Summary		Get a runner
//	@Description	Get information about a specific runner
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			rname	path		string	true	"Runner name"
//	@Success		200		{object}	models.Runner
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners/{rname} [get]
//	@Security		Bearer
func (rc *RunnerController) GetRunner(ctx *gin.Context) {
	// username := ctx.MustGet("username").(string)
	runnerName := ctx.Param("id")
	runner, err := rc.RunnerService.GetRunner(runnerName)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if runner == nil {
		ctx.JSON(http.StatusOK, gin.H{"msg": "runner not found"})
		return
	}

	ctx.JSON(http.StatusOK, runner)
}

// CreateRunner godoc
//
//	@Summary		Create a new runner
//	@Description	Add a runner to the cluster
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			runner	body		models.Runner	true	"New runner"
//	@Success		200		{object}	models.Runner
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners [post]
//	@Security		Bearer
func (rc *RunnerController) CreateRunner(ctx *gin.Context) {
	timestamp := time.Now().UTC()
	username := ctx.MustGet("username").(string)
	var runner models.Runner

	if err := ctx.ShouldBindJSON(&runner); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configMap, err := rc.RunnerService.CreateRunner(runner)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "create", "runners", runner.Name, "failure")
			ctx.JSON(http.StatusConflict, gin.H{"error": "runner already exists, please use a different name"})
			return
		}
		helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "create", "runners", runner.Name, "failure")
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "create", "runners", runner.Name, "success")
	ctx.JSON(http.StatusOK, configMap.Data)
}

// UpdateRunner godoc
//
//	@Summary		Update a runner
//	@Description	Update a runner in the cluster
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			rname	path		string			true	"Runner name"
//	@Param			runner	body		models.Runner	true	"Update runner"
//	@Success		200		{object}	models.Runner
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners/{rname} [patch]
//	@Security		Bearer
func (rc *RunnerController) UpdateRunner(ctx *gin.Context) {
	timestamp := time.Now().UTC()
	username := ctx.MustGet("username").(string)
	var runner models.Runner

	if err := ctx.ShouldBindJSON(&runner); err != nil {
		log.Println(err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configMap, err := rc.RunnerService.UpdateRunner(runner)
	if err != nil {
		if errors.IsNotFound(err) {
			helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "update", "runners", runner.Name, "failure")
			ctx.JSON(http.StatusNotFound, gin.H{"error": "runner doesn't exist"})
			return
		}
		helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "update", "runners", runner.Name, "failure")
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "update", "runners", runner.Name, "success")
	ctx.JSON(http.StatusOK, configMap.Data)
}

// DeleteRunner godoc
//
//	@Summary		Delete a runner
//	@Description	Delete by runner name
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			rname	path		string	true	"Runner name"
//	@Success		204		{object}	models.Runner
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners/{rname} [delete]
//	@Security		Bearer
func (rc *RunnerController) DeleteRunner(ctx *gin.Context) {
	timestamp := time.Now().UTC()
	username := ctx.MustGet("username").(string)
	runnerName := ctx.Param("id")

	err := rc.RunnerService.DeleteRunner(runnerName)
	if err != nil {
		if errors.IsNotFound(err) {
			helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "delete", "runners", runnerName, "failure")
			ctx.JSON(http.StatusConflict, gin.H{"error": "runner doesn't exist"})
			return
		}
		helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "delete", "runners", runnerName, "failure")
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	helpers.CreateElasticSearchLog(rc.ElasticSearch, timestamp, username, ctx.ClientIP(), "delete", "runners", runnerName, "success")
	ctx.JSON(http.StatusOK, gin.H{"msg": "runner deleted successfully"})
}

func (rc *RunnerController) ListAllJobs(ctx *gin.Context) {
	jobsList, err := rc.RunnerService.ListAllJobs()

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

	ctx.JSON(http.StatusOK, jobsList)
}
