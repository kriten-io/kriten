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

type TaskController struct {
	TaskService services.TaskService
	AuthService services.AuthService
}

func NewTaskController(taskservice services.TaskService, as services.AuthService) TaskController {
	return TaskController{
		TaskService: taskservice,
		AuthService: as,
	}
}

func (tc *TaskController) SetTaskRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(tc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(tc.AuthService, "tasks"), tc.ListTasks)
	r.GET("/:id", middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "read"), tc.GetTask)
	r.GET("/:id/schema", middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "read"), tc.GetSchema)
	r.POST("/:id/run", middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "execute"), tc.RunTask)

	r.Use(middlewares.AuthorizationMiddleware(tc.AuthService, "tasks", "write"))
	{
		r.POST("", tc.CreateTask)
		r.PUT("", tc.CreateTask)
		r.PATCH("/:id", tc.UpdateTask)
		r.PUT("/:id", tc.UpdateTask)
		r.DELETE("/:id", tc.DeleteTask)

		{
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

	tasks, total, err := tc.TaskService.ListTasks(authList, params)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", total))
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
//	@Param			id	path	string	true	"Task ID"
//	@Success		200	{object}	models.Task
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id} [get]
//	@Security		Bearer
func (tc *TaskController) GetTask(ctx *gin.Context) {
	taskID, ok := parseUUIDParam(ctx, "id", "task")
	if !ok {
		return
	}

	task, err := tc.TaskService.GetTask(taskID)

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
	var task models.Task

	if err := ctx.ShouldBindJSON(&task); err != nil {
		ctx.Error(errors.New("invalid task payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	taskConfig, err := tc.TaskService.CreateTask(getAuditMetadata(ctx), task)
	if err != nil {
		ctx.Error(err)
		return
	}

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
//	@Router			/tasks/{id} [patch]
//	@Security		Bearer
func (tc *TaskController) UpdateTask(ctx *gin.Context) {
	taskID, ok := parseUUIDParam(ctx, "id", "task")
	if !ok {
		return
	}

	var task models.Task

	if err := ctx.ShouldBindJSON(&task); err != nil {
		ctx.Error(errors.New("invalid task payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	taskConfig, err := tc.TaskService.UpdateTask(getAuditMetadata(ctx), taskID, task)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, taskConfig)
}

// DeleteTask godoc
//
//	@Summary		Delete a task
//	@Description	Delete by task name
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Task ID"
//	@Success		204	{object}	models.ResponseMessage
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id} [delete]
//	@Security		Bearer
func (tc *TaskController) DeleteTask(ctx *gin.Context) {
	taskID, ok := parseUUIDParam(ctx, "id", "task")
	if !ok {
		return
	}

	err := tc.TaskService.DeleteTask(getAuditMetadata(ctx), taskID)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "task deleted successfully"})
}

// GetSchema godoc
//
//	@Summary		Get schema
//	@Description	Get validation schema associated to a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Task ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id}/schema [get]
//	@Security		Bearer
func (tc *TaskController) GetSchema(ctx *gin.Context) {
	taskID, ok := parseUUIDParam(ctx, "id", "task")
	if !ok {
		return
	}

	schema, err := tc.TaskService.GetSchema(taskID)

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
//	@Param			id	path	string	true	"Task ID"
//	@Param			schema	body	map[string]interface{}	true	"New schema"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id}/schema [post]
//	@Security		Bearer
func (tc *TaskController) UpdateSchema(ctx *gin.Context) {
	taskID, ok := parseUUIDParam(ctx, "id", "task")
	if !ok {
		return
	}

	var schema map[string]interface{}

	if err := ctx.BindJSON(&schema); err != nil {
		ctx.Error(errors.New("invalid schema payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	schema, err := tc.TaskService.UpdateSchema(getAuditMetadata(ctx), taskID, schema)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, schema)
}

// DeleteSchema godoc
//
//	@Summary		Delete schema
//	@Description	Remove validation schema associated to a specific task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Task ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/tasks/{id}/schema [delete]
//	@Security		Bearer
func (tc *TaskController) DeleteSchema(ctx *gin.Context) {
	taskID, ok := parseUUIDParam(ctx, "id", "task")
	if !ok {
		return
	}

	err := tc.TaskService.DeleteSchema(getAuditMetadata(ctx), taskID)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "schema deleted successfully"})
}

// RunTask godoc
//
//	@Summary		Run Task
//	@Description	Run Task
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task ID"
//	@Param			evars	body		object	false	"Extra vars"
//	@Success		200		{object}	models.JobMessage
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/tasks/{id}/run [post]
//	@Security		Bearer
func (tc *TaskController) RunTask(ctx *gin.Context) {
	taskID, ok := parseUUIDParam(ctx, "id", "task")
	if !ok {
		return
	}

	username := ctx.MustGet("username").(string)

	extraVars, err := io.ReadAll(ctx.Request.Body)

	if err != nil {
		ctx.Error(errors.New("invalid payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	job, err := tc.TaskService.RunTask(getAuditMetadata(ctx), username, taskID, string(extraVars))

	if err != nil {
		ctx.Error(err)
		return
	}

	if (job.Name != "") && (job.Completed != 0) {
		ctx.JSON(http.StatusOK, job)
		return
	}

	ctx.JSON(http.StatusOK, models.JobMessage{Message: "job created successfully", JobID: job.ID, JobName: job.Name})
}
