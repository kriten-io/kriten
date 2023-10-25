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

type TaskController struct {
	TaskService   services.TaskService
	ElasticSearch helpers.ElasticSearch
	AuthService   services.AuthService
}

func NewTaskController(taskservice services.TaskService, as services.AuthService, es helpers.ElasticSearch) TaskController {
	return TaskController{
		TaskService:   taskservice,
		AuthService:   as,
		ElasticSearch: es,
	}
}

func (tc *TaskController) SetTaskRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(tc.AuthService, "tasks"), tc.ListTasks)
	r.GET("/:id", middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "read"), tc.GetTask)

	r.Use(middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "write"))
	{
		r.POST("", tc.CreateTask)
		r.PUT("", tc.CreateTask)
		r.PATCH("/:id", tc.UpdateTask)
		r.PUT("/:id", tc.UpdateTask)
		r.DELETE("/:id", tc.DeleteTask)
	}

}

// ListTask godoc
//
//	@Summary		List all tasks
//	@Description	List all tasks available on the cluster
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks [get]
//	@Security		Bearer
func (tc *TaskController) ListTasks(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	tasksList, err := tc.TaskService.ListTasks(authList)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(tasksList)))
	if len(tasksList) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	// ctx.Header("Content-range", fmt.Sprintf("%v", len(tasksList)))
	ctx.JSON(http.StatusOK, tasksList)
	// ctx.JSON(http.StatusOK, gin.H{"msg": "tasks list retrieved successfully", "tasks": tasksList})
}

// GetTask godoc
//
//	@Summary		Get a task
//	@Description	Get information about a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task name"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id} [get]
//	@Security		Bearer
func (tc *TaskController) GetTask(ctx *gin.Context) {
	// username := ctx.MustGet("username").(string)
	taskName := ctx.Param("id")
	task, _, err := tc.TaskService.GetTask(taskName)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if task == nil {
		ctx.JSON(http.StatusOK, gin.H{"msg": "task not found"})
		return
	}

	// ctx.JSON(http.StatusOK, gin.H{"msg": "task retrieved successfully", "value": task, "secret": secret})
	ctx.JSON(http.StatusOK, task)
}

// CreateTask godoc
//
//	@Summary		Create a new task
//	@Description	Add a task to the cluster
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			task	body		models.Task	true	"New task"
//	@Success		200		{object}	models.Task
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/tasks [post]
//	@Security		Bearer
func (tc *TaskController) CreateTask(ctx *gin.Context) {
	timestamp := time.Now().UTC()
	username := ctx.MustGet("username").(string)
	var task models.Task

	if err := ctx.ShouldBindJSON(&task); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configMap, _, err := tc.TaskService.CreateTask(task)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "create", "tasks", task.Name, "failure")
			ctx.JSON(http.StatusConflict, gin.H{"error": "task already exists, please use a different name"})
			return
		}
		helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "create", "tasks", task.Name, "failure")
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "create", "tasks", task.Name, "success")
	ctx.JSON(http.StatusOK, configMap.Data)
}

// UpdateTask godoc
//
//	@Summary		Update a task
//	@Description	Update a task in the cluster
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string		true	"Task name"
//	@Param			task	body		models.Task	true	"Update task"
//	@Success		200		{object}	models.Task
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/tasks/{id} [patch]
//	@Security		Bearer
func (tc *TaskController) UpdateTask(ctx *gin.Context) {
	timestamp := time.Now().UTC()
	username := ctx.MustGet("username").(string)
	var task models.Task

	if err := ctx.ShouldBindJSON(&task); err != nil {
		log.Println(err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configMap, _, err := tc.TaskService.UpdateTask(task)
	if err != nil {
		if errors.IsNotFound(err) {
			helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "update", "tasks", task.Name, "failure")
			ctx.JSON(http.StatusConflict, gin.H{"error": "task doesn't exist"})
			return
		}
		helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "update", "tasks", task.Name, "failure")
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "update", "tasks", task.Name, "success")
	ctx.JSON(http.StatusOK, configMap.Data)
}

// DeleteTask godoc
//
//	@Summary		Delete a task
//	@Description	Delete by task name
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task name"
//	@Success		204	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id} [delete]
//	@Security		Bearer
func (tc *TaskController) DeleteTask(ctx *gin.Context) {
	timestamp := time.Now().UTC()
	username := ctx.MustGet("username").(string)
	taskName := ctx.Param("id")

	err := tc.TaskService.DeleteTask(taskName)
	if err != nil {
		if errors.IsNotFound(err) {
			helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "delete", "tasks", taskName, "failure")
			ctx.JSON(http.StatusConflict, gin.H{"error": "task doesn't exist"})
			return
		}
		helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "delete", "tasks", taskName, "failure")
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	helpers.CreateElasticSearchLog(tc.ElasticSearch, timestamp, username, ctx.ClientIP(), "delete", "tasks", taskName, "success")
	ctx.JSON(http.StatusOK, gin.H{"msg": "task deleted successfully"})
}
