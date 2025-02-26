package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/errors"
)

type TaskController struct {
	TaskService   services.TaskService
	AuthService   services.AuthService
	AuditService  services.AuditService
	AuditCategory string
}

func NewTaskController(taskservice services.TaskService, as services.AuthService, als services.AuditService) TaskController {
	return TaskController{
		TaskService:   taskservice,
		AuthService:   as,
		AuditService:  als,
		AuditCategory: "tasks",
	}
}

func (tc *TaskController) SetTaskRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(tc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(tc.AuthService, "tasks"), tc.ListTasks)
	r.GET("/:id", middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "read"), tc.GetTask)

	r.Use(middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "write"))
	{
		r.POST("", tc.CreateTask)
		r.PUT("", tc.CreateTask)
		r.PATCH("/:id", tc.UpdateTask)
		r.PUT("/:id", tc.UpdateTask)
		r.DELETE("/:id", tc.DeleteTask)

		{
			r.GET("/:id/schema", tc.GetSchema)
			r.POST("/:id/schema", tc.UpdateSchema)
			r.PUT("/:id/schema", tc.UpdateSchema)
			r.DELETE("/:id/schema", tc.DeleteSchema)
		}
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
	audit := tc.AuditService.InitialiseAuditLog(ctx, "list", tc.AuditCategory, "*")
	authList := ctx.MustGet("authList").([]string)

	tasksList, err := tc.TaskService.ListTasks(authList)

	if err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	ctx.Header("Content-range", fmt.Sprintf("%v", len(tasksList)))
	if len(tasksList) == 0 {
		var arr [0]int
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	// ctx.Header("Content-range", fmt.Sprintf("%v", len(tasksList)))
	tc.AuditService.CreateAudit(audit)
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
	taskName := ctx.Param("id")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "get", tc.AuditCategory, taskName)
	// username := ctx.MustGet("username").(string)
	task, err := tc.TaskService.GetTask(taskName)

	if err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if task == nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, gin.H{"msg": "task not found"})
		return
	}
	audit.Status = "success"

	// ctx.JSON(http.StatusOK, gin.H{"msg": "task retrieved successfully", "value": task, "secret": secret})
	tc.AuditService.CreateAudit(audit)
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
	audit := tc.AuditService.InitialiseAuditLog(ctx, "create", tc.AuditCategory, "*")
	var task models.Task

	if err := ctx.ShouldBindJSON(&task); err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit.EventTarget = task.Name

	taskConfig, err := tc.TaskService.CreateTask(task)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			tc.AuditService.CreateAudit(audit)
			ctx.JSON(http.StatusConflict, gin.H{"error": "task already exists, please use a different name"})
			return
		}
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, taskConfig)
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
	taskName := ctx.Param("id")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "update", tc.AuditCategory, taskName)
	var task models.Task

	if err := ctx.ShouldBindJSON(&task); err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskConfig, err := tc.TaskService.UpdateTask(task)
	if err != nil {
		if errors.IsNotFound(err) {
			tc.AuditService.CreateAudit(audit)
			ctx.JSON(http.StatusConflict, gin.H{"error": "task doesn't exist"})
			return
		}
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, taskConfig)
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
	taskName := ctx.Param("id")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "delete", tc.AuditCategory, taskName)

	err := tc.TaskService.DeleteTask(taskName)
	if err != nil {
		if errors.IsNotFound(err) {
			tc.AuditService.CreateAudit(audit)
			ctx.JSON(http.StatusConflict, gin.H{"error": "task doesn't exist"})
			return
		}
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "task deleted successfully"})
}

// GetSchema godoc
//
//	@Summary		Get schema
//	@Description	Get validation schema associated to a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id}/schema [get]
//	@Security		Bearer
func (tc *TaskController) GetSchema(ctx *gin.Context) {
	taskName := ctx.Param("id")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "get_schema", tc.AuditCategory, taskName)
	schema, err := tc.TaskService.GetSchema(taskName)

	if err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"

	if schema == nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, gin.H{"msg": "schema not found"})
		return
	}

	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, schema)
}

// UpdateSchema godoc
//
//	@Summary		Update schema
//	@Description	Add or Update validation schema associated to a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task name"
//	@Param			schema	body	map[string]interface{}	true	"New schema"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id}/schema [post]
//	@Security		Bearer
func (tc *TaskController) UpdateSchema(ctx *gin.Context) {
	taskName := ctx.Param("id")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "update_schema", tc.AuditCategory, taskName)
	var schema map[string]interface{}

	if err := ctx.BindJSON(&schema); err != nil {
		log.Println(err)
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	schema, err := tc.TaskService.UpdateSchema(taskName, schema)
	if err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, schema)
}

// DeleteSchema godoc
//
//	@Summary		Delete schema
//	@Description	Remove validation schema associated to a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id}/schema [delete]
//	@Security		Bearer
func (tc *TaskController) DeleteSchema(ctx *gin.Context) {
	taskName := ctx.Param("id")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "delete_schema", tc.AuditCategory, taskName)

	err := tc.TaskService.DeleteSchema(taskName)

	if err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "schema deleted successfully"})
}
