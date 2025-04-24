package controllers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

type WebhookController struct {
	WebhookService services.WebhookService
	JobService     services.JobService
	AuthService    services.AuthService
	providers      []string
	AuditService   services.AuditService
	AuditCategory  string
}

func NewWebhookController(
	ws services.WebhookService,
	js services.JobService,
	as services.AuthService,
	als services.AuditService,
	p []string,
) WebhookController {
	return WebhookController{
		WebhookService: ws,
		JobService:     js,
		AuthService:    as,
		providers:      p,
		AuditService:   als,
		AuditCategory:  "webHooks",
	}
}

func (wc *WebhookController) SetWebhookRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(wc.AuthService, config.JWT))

	// Authorizations is set in the svc, only returning own tokens
	r.GET("", wc.ListWebhooks)

	r.GET("/all", middlewares.SetAuthorizationListMiddleware(wc.AuthService, "webHooks"), wc.ListAllWebhooks)
	r.GET("/:id", middlewares.AuthorizationMiddleware(wc.AuthService, "webHooks", "read"), wc.GetWebhook)

	r.POST("", wc.CreateWebhook)
	r.PUT("", wc.CreateWebhook)

	r.POST("/run/:id", middlewares.AuthorizationMiddleware(wc.AuthService, "jobs", "write"), wc.RunWebhook)

	r.Use(middlewares.AuthorizationMiddleware(wc.AuthService, "webHooks", "write"))
	{
		//r.PATCH("/:id", wc.UpdateWebhooks)
		//r.PUT("/:id", wc.UpdateWebhooks)
		r.DELETE("/:id", wc.DeleteWebhook)
	}
}

// ListWebhooks godoc
//
//	@Summary		List own webHooks
//	@Description	List own webHooks available on the cluster
//	@Tags			api_tokens
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.Webhook
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/webhooks [get]
//	@Security		Bearer
func (wc *WebhookController) ListWebhooks(ctx *gin.Context) {
	// audit := wc.AuditService.InitialiseAuditLog(ctx, "list", wc.AuditCategory, "*")
	userid := ctx.MustGet("userID").(uuid.UUID)
	webHooks, err := wc.WebhookService.ListWebhooks(userid)

	if err != nil {
		// wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// audit.Status = "success"
	ctx.Header("Content-range", fmt.Sprintf("%v", len(webHooks)))
	if len(webHooks) == 0 {
		var arr [0]int
		// wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	// wc.AuditService.CreateAudit(audit)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, webHooks)
}

// ListAllWebhooks godoc
//
//	@Summary		List all webHooks
//	@Description	List all webHooks available on the cluster
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.Webhook
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/webhooks/all [get]
//	@Security		Bearer
func (wc *WebhookController) ListAllWebhooks(ctx *gin.Context) {
	// audit := wc.AuditService.InitialiseAuditLog(ctx, "list", wc.AuditCategory, "*")
	authList := ctx.MustGet("authList").([]string)
	webHooks, err := wc.WebhookService.ListAllWebhooks(authList)

	if err != nil {
		// wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// audit.Status = "success"
	ctx.Header("Content-range", fmt.Sprintf("%v", len(webHooks)))
	if len(webHooks) == 0 {
		var arr [0]int
		// wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	// wc.AuditService.CreateAudit(audit)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, webHooks)
}

// GetWebhook godoc
//
//	@Summary		Get a webHook
//	@Description	Get information about a specific webHook
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Webhook ID"
//	@Success		200	{object}	models.Webhook
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/webhooks/{id} [get]
//	@Security		Bearer
func (wc *WebhookController) GetWebhook(ctx *gin.Context) {
	webhookID := ctx.Param("id")
	// audit := wc.AuditService.InitialiseAuditLog(ctx, "get", wc.AuditCategory, webhookID)
	webhook, err := wc.WebhookService.GetWebhook(webhookID)

	if err != nil {
		// wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// audit.Status = "success"
	// wc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, webhook)
}

// CreateWebhook godoc
//
//	@Summary		Create a new webhook
//	@Description	Add a webhook to the cluster
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Param			webhook	body		models.Webhook	true	"New Webhook"
//	@Success		200		{object}	models.Webhook
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/webhooks [post]
//	@Security		Bearer
func (wc *WebhookController) CreateWebhook(ctx *gin.Context) {
	userid := ctx.MustGet("userID").(uuid.UUID)
	audit := wc.AuditService.InitialiseAuditLog(ctx, "create", wc.AuditCategory, "*")
	var webhook models.Webhook

	if err := ctx.ShouldBindJSON(&webhook); err != nil {
		wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	webhook.Owner = userid

	webhook, err := wc.WebhookService.CreateWebhook(webhook)
	if err != nil {
		wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// if audit.EventTarget == "" {
	// 	audit.EventTarget = apiToken.Key
	// }

	audit.Status = "success"
	ctx.JSON(http.StatusOK, webhook)
}

// DeleteWebhook godoc
//
//	@Summary		Delete a webhook
//	@Description	Delete by webhook ID
//	@Tags			webhook
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Webhook ID"
//	@Success		204	{object}	models.Webhook
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/webhooks/{id} [delete]
//	@Security		Bearer
func (wc *WebhookController) DeleteWebhook(ctx *gin.Context) {
	webhookID := ctx.Param("id")
	audit := wc.AuditService.InitialiseAuditLog(ctx, "delete", wc.AuditCategory, webhookID)

	err := wc.WebhookService.DeleteWebhook(webhookID)
	if err != nil {
		wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	wc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "webhook deleted successfully"})
}

// RunWebhook godoc
//
//	@Summary		Run webhook
//	@Description	Execute Kriten job via webhook
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Param			id	    path		string	true	"Webhook ID"
//	@Param			evars	body		object	false	"Extra vars"
//	@Success		200		{object}	models.Job
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/webhooks/run/{id} [post]
//	@Security		Signature
func (wc *WebhookController) RunWebhook(ctx *gin.Context) {
	webhookID := ctx.Param("id")
	taskID := ctx.MustGet("taskID").(string)
	username := ctx.MustGet("username").(string)

	audit := wc.AuditService.InitialiseAuditLog(ctx, "run", wc.AuditCategory, webhookID)
	audit.Status = "success"
	wc.AuditService.CreateAudit(audit)

	extraVars, err := io.ReadAll(ctx.Request.Body)

	if err != nil {
		wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	job, err := wc.JobService.CreateJob(username, taskID, string(extraVars))

	if err != nil {
		wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"

	if (job.ID != "") && (job.Completed != 0) {
		//ctx.JSON(http.StatusOK, gin.H{"id": jobID, "json_data": sync.JsonData})
		wc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, job)
		return
	}

	wc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "job created successfully", "id": job.ID})
}
