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
	r.GET("/:name", middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "read"), tc.GetTask)
	r.GET("/:name/schema", middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "read"), tc.GetSchema)

	r.Use(middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "write"))
	{
		r.POST("", tc.CreateTask)
		r.PUT("", tc.CreateTask)
		r.PATCH("/:name", tc.UpdateTask)
		r.PUT("/:name", tc.UpdateTask)
		r.DELETE("/:name", tc.DeleteTask)

		{
			r.POST("/:name/schema", tc.UpdateSchema)
			r.PUT("/:name/schema", tc.UpdateSchema)
			r.DELETE("/:name/schema", tc.DeleteSchema)
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
//	@Param			limit	query		int		false	"Maximum number of tasks to return (default 100)"
//	@Param			offset	query		int		false	"Number of tasks to skip (default 0)"
//	@Param			name	query		string	false	"Filter by task name"
//	@Success		200	{array}		models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks [get]
//	@Security		Bearer
func (tc *TaskController) ListTasks(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)
	var params models.TaskQueryParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		ctx.Error(errors.New("invalid query parameters"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	tasks, err := tc.TaskService.ListTasks(authList, params)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(tasks)))
	if len(tasks) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.JSON(http.StatusOK, tasks)
}

// GetTask godoc
//
//	@Summary		Get a task
//	@Description	Get information about a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			name	path	string	true	"Task name"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{name} [get]
//	@Security		Bearer
func (tc *TaskController) GetTask(ctx *gin.Context) {
	taskName := ctx.Param("name")
	task, err := tc.TaskService.GetTask(taskName)

	if err != nil {
		ctx.Error(err)
		return
	}

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
		ctx.Error(errors.New("invalid task payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	audit.EventTarget = task.Name

	taskConfig, err := tc.TaskService.CreateTask(task)
	if err != nil {
		ctx.Error(err)
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
//	@Param			name	path		string		true	"Task name"
//	@Param			task	body		models.Task	true	"Update task"
//	@Success		200		{object}	models.Task
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/tasks/{name} [patch]
//	@Security		Bearer
func (tc *TaskController) UpdateTask(ctx *gin.Context) {
	taskName := ctx.Param("name")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "update", tc.AuditCategory, taskName)
	var task models.Task

	if err := ctx.ShouldBindJSON(&task); err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid task payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	taskConfig, err := tc.TaskService.UpdateTask(task)
	if err != nil {
		ctx.Error(err)
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
//	@Param			name	path	string	true	"Task name"
//	@Success		204	{object}	models.ResponseMessage
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{name} [delete]
//	@Security		Bearer
func (tc *TaskController) DeleteTask(ctx *gin.Context) {
	taskName := ctx.Param("name")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "delete", tc.AuditCategory, taskName)

	err := tc.TaskService.DeleteTask(taskName)
	if err != nil {
		ctx.Error(err)
		return
	}

	audit.Status = "success"
	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "task deleted successfully"})
}

// GetSchema godoc
//
//	@Summary		Get schema
//	@Description	Get validation schema associated to a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			name	path	string	true	"Task name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{name}/schema [get]
//	@Security		Bearer
func (tc *TaskController) GetSchema(ctx *gin.Context) {
	taskName := ctx.Param("name")
	schema, err := tc.TaskService.GetSchema(taskName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, schema)
}

// UpdateSchema godoc
//
//	@Summary		Update schema
//	@Description	Add or Update validation schema associated to a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			name	path	string	true	"Task name"
//	@Param			schema	body	map[string]interface{}	true	"New schema"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{name}/schema [post]
//	@Security		Bearer
func (tc *TaskController) UpdateSchema(ctx *gin.Context) {
	taskName := ctx.Param("name")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "update_schema", tc.AuditCategory, taskName)
	var schema map[string]interface{}

	if err := ctx.BindJSON(&schema); err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.Error(errors.New("invalid schema payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	schema, err := tc.TaskService.UpdateSchema(taskName, schema)
	if err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.Error(err)
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
//	@Param			name	path	string	true	"Task name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{name}/schema [delete]
//	@Security		Bearer
func (tc *TaskController) DeleteSchema(ctx *gin.Context) {
	taskName := ctx.Param("name")
	audit := tc.AuditService.InitialiseAuditLog(ctx, "delete_schema", tc.AuditCategory, taskName)

	err := tc.TaskService.DeleteSchema(taskName)

	if err != nil {
		tc.AuditService.CreateAudit(audit)
		ctx.Error(err)
		return
	}

	audit.Status = "success"
	tc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "schema deleted successfully"})
}
