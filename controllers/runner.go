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

type RunnerController struct {
	RunnerService services.RunnerService
	AuthService   services.AuthService
}

func NewRunnerController(rs services.RunnerService, as services.AuthService) RunnerController {
	return RunnerController{
		RunnerService: rs,
		AuthService:   as,
	}
}

func (rc *RunnerController) SetRunnerRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(rc.AuthService, config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(rc.AuthService, "runners"), rc.ListRunners)
	r.GET("/:name", middlewares.AuthorizationMiddleware(rc.AuthService, "runners", "read"), rc.GetRunner)
	r.Use(middlewares.AuthorizationMiddleware(rc.AuthService, "runners", "write"))
	{
		r.POST("", rc.CreateRunner)
		r.PUT("", rc.CreateRunner)
		r.PATCH("/:name", rc.UpdateRunner)
		r.PUT("/:name", rc.UpdateRunner)
		r.DELETE("/:name", rc.DeleteRunner)

		{
			r.GET("/:name/secret", rc.GetSecret)
			r.POST("/:name/secret", rc.UpdateSecret)
			r.PUT("/:name/secret", rc.UpdateSecret)
			r.DELETE("/:name/secret", rc.DeleteSecret)
		}
	}

}

// ListRunners godoc
//
//	@Summary		List all runners
//	@Description	List all runners available on the cluster
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			limit	query		int		false	"Maximum number of runners to return (default 100)"
//	@Param			offset	query		int		false	"Number of runners to skip (default 0)"
//	@Param			name	query		string	false	"Filter by runner name"
//	@Success		200	{array}		models.Runner
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/runners [get]
//	@Security		Bearer
func (rc *RunnerController) ListRunners(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)

	var params models.RunnerQueryParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		ctx.Error(errors.New("invalid query parameters"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if params.Limit == 0 {
		params.Limit = 100
	}

	runnersList, total, err := rc.RunnerService.ListRunners(authList, params)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", total))
	if len(runnersList) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

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
//	@Param			name	path		string	true	"Runner name"
//	@Success		200		{object}	models.Runner
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners/{name} [get]
//	@Security		Bearer
func (rc *RunnerController) GetRunner(ctx *gin.Context) {
	runnerName := ctx.Param("name")

	runner, err := rc.RunnerService.GetRunner(runnerName)
	if err != nil {
		ctx.Error(err)
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
//	@Failure		409		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners [post]
//	@Security		Bearer
func (rc *RunnerController) CreateRunner(ctx *gin.Context) {
	var runner models.Runner

	if err := ctx.ShouldBindJSON(&runner); err != nil {
		ctx.Error(errors.New("invalid runner payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	runnerData, err := rc.RunnerService.CreateRunner(getActor(ctx), runner)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, runnerData)
}

// UpdateRunner godoc
//
//	@Summary		Update a runner
//	@Description	Update a runner in the cluster
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string			true	"Runner name"
//	@Param			runner	body		models.Runner	true	"Update runner"
//	@Success		200		{object}	models.Runner
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners/{name} [patch]
//	@Security		Bearer
func (rc *RunnerController) UpdateRunner(ctx *gin.Context) {
	runnerName := ctx.Param("name")
	var runner models.Runner

	if err := ctx.ShouldBindJSON(&runner); err != nil {
		ctx.Error(fmt.Errorf("runner '%s': invalid runner payload", runnerName))
		ctx.Status(http.StatusBadRequest)
		return
	}

	if runner.Name != runnerName {
		ctx.Error(fmt.Errorf("runner '%s': name in url does not match runner name in payload", runnerName))
		ctx.Status(http.StatusBadRequest)
		return
	}

	runnerData, err := rc.RunnerService.UpdateRunner(getActor(ctx), runner)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, runnerData)
}

// DeleteRunner godoc
//
//	@Summary		Delete a runner
//	@Description	Delete by runner name
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Runner name"
//	@Success		200		{object}	models.ResponseMessage
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		409		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/runners/{name} [delete]
//	@Security		Bearer
func (rc *RunnerController) DeleteRunner(ctx *gin.Context) {
	runnerName := ctx.Param("name")

	err := rc.RunnerService.DeleteRunner(getActor(ctx), runnerName)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "runner deleted successfully"})
}

// GetSecret godoc
//
//	@Summary		Get secret
//	@Description	Get secret associated with runner (passwords are obfuscated)
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Runner name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/runners/{name}/secret [get]
//	@Security		Bearer
func (rc *RunnerController) GetSecret(ctx *gin.Context) {
	runnerName := ctx.Param("name")
	secret, err := rc.RunnerService.GetSecret(runnerName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, secret)
}

// GetSecret godoc
//
//	@Summary		Update secret
//	@Description	Update secret associated with runner
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Runner name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/runners/{name}/secret [get]
//	@Security		Bearer
func (rc *RunnerController) UpdateSecret(ctx *gin.Context) {
	runnerName := ctx.Param("name")
	var secret map[string]string

	if err := ctx.BindJSON(&secret); err != nil {
		ctx.Error(errors.New("invalid secrets payload"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	secretStored, err := rc.RunnerService.UpdateSecret(getActor(ctx), runnerName, secret)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, secretStored)
}

// DeleteSecret godoc
//
//	@Summary		Delete secret
//	@Description	Remove secret associated with runner
//	@Tags			runners
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string	true	"Runner name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/runners/{name}/schema [delete]
//	@Security		Bearer
func (rc *RunnerController) DeleteSecret(ctx *gin.Context) {
	runnerName := ctx.Param("name")

	err := rc.RunnerService.DeleteSecret(getActor(ctx), runnerName)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "secret deleted successfully"})
}
