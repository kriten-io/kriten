package controllers

import (
	"fmt"
	"kriten/config"
	"kriten/middlewares"
	"kriten/models"
	"kriten/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DeploymentController struct {
	DeploymentService services.DeploymentService
	AuthService       services.AuthService
	AuditService      services.AuditService
	AuditCategory     string
}

func NewDeploymentController(js services.DeploymentService, as services.AuthService, als services.AuditService) DeploymentController {
	return DeploymentController{
		DeploymentService: js,
		AuthService:       as,
		AuditService:      als,
		AuditCategory:     "deployments",
	}
}

func (dc *DeploymentController) SetDeploymentRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

	r.GET("", middlewares.SetAuthorizationListMiddleware(dc.AuthService, "deployments"), dc.ListDeployments)
	r.GET("/:id", middlewares.AuthorizationMiddleware(dc.AuthService, "deployments", "read"), dc.GetDeployment)
	r.GET("/:id/schema", middlewares.AuthorizationMiddleware(dc.AuthService, "deployments", "read"), dc.GetSchema)

	r.Use(middlewares.AuthorizationMiddleware(dc.AuthService, "deployments", "write"))
	{
		r.POST("", dc.CreateDeployment)
		r.PUT("", dc.CreateDeployment)
		r.PATCH("/:id", dc.UpdateDeployment)
		r.PUT("/:id", dc.UpdateDeployment)
		r.DELETE("/:id", dc.DeleteDeployment)
	}

}

// ListDeployments godoc
//
//	@Summary		List all deployments
//	@Description	List all deployments
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.Deployment
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/deployments [get]
//	@Security		Bearer
func (dc *DeploymentController) ListDeployments(ctx *gin.Context) {
	audit := dc.AuditService.InitialiseAuditLog(ctx, "list", dc.AuditCategory, "*")
	authList := ctx.MustGet("authList").([]string)

	jobsList, err := dc.DeploymentService.ListDeployments(authList)

	if err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	ctx.Header("Content-range", fmt.Sprintf("%v", len(jobsList)))
	if len(jobsList) == 0 {
		var arr [0]int
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	dc.AuditService.CreateAudit(audit)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, jobsList)
}

// GetDeployment godoc
//
//	@Summary		Get job info
//	@Description	Get information about a specific job
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Deployment  id"
//	@Success		200	{object}	models.Deployment
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/deployments/{id} [get]
//	@Security		Bearer
func (dc *DeploymentController) GetDeployment(ctx *gin.Context) {
	jobName := ctx.Param("id")
	audit := dc.AuditService.InitialiseAuditLog(ctx, "get", dc.AuditCategory, jobName)
	job, err := dc.DeploymentService.GetDeployment(jobName)

	if err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	dc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, job)
}

// CreateDeployment godoc
//
//	@Summary		Create a new job
//	@Description	Add a job to the cluster
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			deployment	body		models.Deployment	true	"New deployment"
//	@Success		200		{object}	models.Deployment
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/deployments [post]
//	@Security		Bearer
func (dc *DeploymentController) CreateDeployment(ctx *gin.Context) {
	var deployment models.Deployment
	audit := dc.AuditService.InitialiseAuditLog(ctx, "create", dc.AuditCategory, "*")
	username := ctx.MustGet("username").(string)

	if err := ctx.ShouldBindJSON(&deployment); err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit.EventTarget = deployment.Task

	deployment.Owner = username
	job, err := dc.DeploymentService.CreateDeployment(deployment)

	if err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	if job.Name != "" {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, job)
		return
	}

	dc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "job created successfully", "id": job.Name})
}

// UpdateDeployment godoc
//
//	@Summary		Update a deployment
//	@Description	Update a deployment in the cluster
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			deployment	body		models.Deployment	true	"Update Deployment"
//	@Success		200		{object}	models.Deployment
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/deployments/ [patch]
//	@Security		Bearer
func (dc *DeploymentController) UpdateDeployment(ctx *gin.Context) {
	var deployment models.Deployment
	var err error
	id := ctx.Param("id")
	username := ctx.MustGet("username").(string)
	audit := dc.AuditService.InitialiseAuditLog(ctx, "update", dc.AuditCategory, id)

	if err := ctx.ShouldBindJSON(&deployment); err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deployment.Owner = username
	deployment, err = dc.DeploymentService.UpdateDeployment(deployment)
	if err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	dc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, deployment)
}

// DeleteDeployment godoc
//
//	@Summary		Delete a Deployment
//	@Description	Delete by Deployment ID
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Deployment ID"
//	@Success		204	{object}	models.Deployment
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/deployments/{id} [delete]
//	@Security		Bearer
func (dc *DeploymentController) DeleteDeployment(ctx *gin.Context) {
	groupID := ctx.Param("id")
	audit := dc.AuditService.InitialiseAuditLog(ctx, "delete", dc.AuditCategory, groupID)

	err := dc.DeploymentService.DeleteDeployment(groupID)
	if err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	dc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "deployment deleted successfully"})
}

// GetSchema godoc
//
//	@Summary		Get schema
//	@Description	Get schema for the job info and input parameters
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Task  name"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/deployments/{id}/schema [get]
//	@Security		Bearer
func (dc *DeploymentController) GetSchema(ctx *gin.Context) {
	taskName := ctx.Param("id")
	audit := dc.AuditService.InitialiseAuditLog(ctx, "get_schema", dc.AuditCategory, taskName)
	schema, err := dc.DeploymentService.GetSchema(taskName)

	if err != nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"

	if schema == nil {
		dc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, gin.H{"msg": "schema not found"})
		return
	}

	dc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, schema)
}
