package controllers

import (
	"fmt"
	"kriten/config"
	"kriten/middlewares"
	"kriten/models"
	"kriten/services"
	"net/http"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

type ApiTokenController struct {
	ApiTokenService services.ApiTokenService
	AuthService     services.AuthService
	providers       []string
	AuditService    services.AuditService
	AuditCategory   string
}

func NewApiTokenController(apiTokenService services.ApiTokenService, as services.AuthService, als services.AuditService, p []string) ApiTokenController {
	return ApiTokenController{
		ApiTokenService: apiTokenService,
		AuthService:     as,
		providers:       p,
		AuditService:    als,
		AuditCategory:   "apiTokens",
	}
}

func (uc *ApiTokenController) SetApiTokenRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(uc.AuthService, config.JWT))

	// Authorizations is set in the svc, only returning own tokens
	r.GET("", uc.ListApiTokens)

	r.GET("/all", middlewares.SetAuthorizationListMiddleware(uc.AuthService, "apiTokens"), uc.ListAllApiTokens)
	r.GET("/:id", middlewares.AuthorizationMiddleware(uc.AuthService, "apiTokens", "read"), uc.GetApiToken)

	r.POST("", uc.CreateApiToken)
	r.PUT("", uc.CreateApiToken)

	r.Use(middlewares.AuthorizationMiddleware(uc.AuthService, "apiTokens", "write"))
	{
		r.PATCH("/:id", uc.UpdateApiToken)
		r.PUT("/:id", uc.UpdateApiToken)
		r.DELETE("/:id", uc.DeleteApiToken)
	}
}

// ListApiTokens godoc
//
//	@Summary		List own apiTokens
//	@Description	List own apiTokens available on the cluster
//	@Tags			api_tokens
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.ApiToken
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api_tokens [get]
//	@Security		Bearer
func (uc *ApiTokenController) ListApiTokens(ctx *gin.Context) {
	audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, "*")
	userid := ctx.MustGet("userID").(uuid.UUID)
	apiTokens, err := uc.ApiTokenService.ListApiTokens(userid)

	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	ctx.Header("Content-range", fmt.Sprintf("%v", len(apiTokens)))
	if len(apiTokens) == 0 {
		var arr [0]int
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	uc.AuditService.CreateAudit(audit)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, apiTokens)
}

// ListAllApiTokens godoc
//
//	@Summary		List all apiTokens
//	@Description	List all apiTokens available on the cluster
//	@Tags			api_tokens
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.ApiToken
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api_tokens/all [get]
//	@Security		Bearer
func (uc *ApiTokenController) ListAllApiTokens(ctx *gin.Context) {
	audit := uc.AuditService.InitialiseAuditLog(ctx, "list", uc.AuditCategory, "*")
	authList := ctx.MustGet("authList").([]string)
	apiTokens, err := uc.ApiTokenService.ListAllApiTokens(authList)

	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	ctx.Header("Content-range", fmt.Sprintf("%v", len(apiTokens)))
	if len(apiTokens) == 0 {
		var arr [0]int
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusOK, arr)
		return
	}

	uc.AuditService.CreateAudit(audit)
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, apiTokens)
}

// GetApiToken godoc
//
//	@Summary		Get a apiToken
//	@Description	Get information about a specific apiToken
//	@Tags			api_tokens
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"ApiToken ID"
//	@Success		200	{object}	models.ApiToken
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api_tokens/{id} [get]
//	@Security		Bearer
func (uc *ApiTokenController) GetApiToken(ctx *gin.Context) {
	apiTokenID := ctx.Param("id")
	audit := uc.AuditService.InitialiseAuditLog(ctx, "get", uc.AuditCategory, apiTokenID)
	apiToken, err := uc.ApiTokenService.GetApiToken(apiTokenID)

	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	audit.Status = "success"
	uc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, apiToken)
}

// CreateApiToken godoc
//
//	@Summary		Create a new apiToken
//	@Description	Add a apiToken to the cluster
//	@Tags			api_tokens
//	@Accept			json
//	@Produce		json
//	@Param			apiToken	body		models.ApiToken	true	"New apiToken"
//	@Success		200		{object}	models.ApiToken
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/api_tokens [post]
//	@Security		Bearer
func (atc *ApiTokenController) CreateApiToken(ctx *gin.Context) {
	userid := ctx.MustGet("userID").(uuid.UUID)
	audit := atc.AuditService.InitialiseAuditLog(ctx, "create", atc.AuditCategory, "*")
	var apiToken models.ApiToken

	if err := ctx.ShouldBindJSON(&apiToken); err != nil {
		atc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	audit.EventTarget = apiToken.Key
	apiToken.Owner = userid

	apiToken, err := atc.ApiTokenService.CreateApiToken(apiToken)
	if err != nil {
		atc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if audit.EventTarget == "" {
		audit.EventTarget = apiToken.Key
	}

	audit.Status = "success"
	ctx.JSON(http.StatusOK, apiToken)
}

// UpdateApiToken godoc
//
//	@Summary		Update a apiToken
//	@Description	Update a apiToken in the cluster
//	@Tags			api_tokens
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string		true	"ApiToken ID"
//	@Param			apiToken	body		models.ApiToken	true	"Update apiToken"
//	@Success		200		{object}	models.ApiToken
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/api_tokens/{id} [patch]
//	@Security		Bearer
func (uc *ApiTokenController) UpdateApiToken(ctx *gin.Context) {
	apiTokenID := ctx.Param("id")
	audit := uc.AuditService.InitialiseAuditLog(ctx, "update", uc.AuditCategory, apiTokenID)
	var apiToken models.ApiToken
	var err error

	if err := ctx.ShouldBindJSON(&apiToken); err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	apiToken.ID, err = uuid.FromString(apiTokenID)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	apiToken, err = uc.ApiTokenService.UpdateApiToken(apiToken)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	uc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, apiToken)
}

// DeleteApiToken godoc
//
//	@Summary		Delete a apiToken
//	@Description	Delete by apiToken ID
//	@Tags			api_tokens
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"ApiToken ID"
//	@Success		204	{object}	models.ApiToken
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api_tokens/{id} [delete]
//	@Security		Bearer
func (uc *ApiTokenController) DeleteApiToken(ctx *gin.Context) {
	apiTokenID := ctx.Param("id")
	audit := uc.AuditService.InitialiseAuditLog(ctx, "delete", uc.AuditCategory, apiTokenID)

	err := uc.ApiTokenService.DeleteApiToken(apiTokenID)
	if err != nil {
		uc.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit.Status = "success"
	uc.AuditService.CreateAudit(audit)
	ctx.JSON(http.StatusOK, gin.H{"msg": "apiToken deleted successfully"})
}
