package controllers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

type ApiTokenController struct {
	ApiTokenService services.ApiTokenService
	AuthService     services.AuthService
	providers       []string
}

func NewApiTokenController(apiTokenService services.ApiTokenService, as services.AuthService, p []string) ApiTokenController {
	return ApiTokenController{
		ApiTokenService: apiTokenService,
		AuthService:     as,
		providers:       p,
	}
}

func (atc *ApiTokenController) SetApiTokenRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(atc.AuthService, config.JWT))

	// Authorizations is set in the svc, only returning own tokens
	r.GET("", atc.ListApiTokens)

	r.GET("/all", middlewares.SetAuthorizationListMiddleware(atc.AuthService, "apiTokens"), atc.ListAllApiTokens)
	r.GET("/:id", middlewares.AuthorizationMiddleware(atc.AuthService, "apiTokens", "read"), atc.GetApiToken)

	r.POST("", atc.CreateApiToken)
	r.PUT("", atc.CreateApiToken)

	r.Use(middlewares.AuthorizationMiddleware(atc.AuthService, "apiTokens", "write"))
	{
		r.PATCH("/:id", atc.UpdateApiToken)
		r.PUT("/:id", atc.UpdateApiToken)
		r.DELETE("/:id", atc.DeleteApiToken)
	}
}

// ListApiTokens godoc
//
//	@Summary		List own apiTokens
//	@Description	List own apiTokens available on the cluster
//	@Tags			api-tokens
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.ApiToken
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api-tokens [get]
//	@Security		Bearer
func (atc *ApiTokenController) ListApiTokens(ctx *gin.Context) {
	userid := ctx.MustGet("userID").(uuid.UUID)
	apiTokens, err := atc.ApiTokenService.ListApiTokens(userid)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(apiTokens)))
	if len(apiTokens) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, apiTokens)
}

// ListAllApiTokens godoc
//
//	@Summary		List all apiTokens
//	@Description	List all apiTokens available on the cluster
//	@Tags			api-tokens
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.ApiToken
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api-tokens/all [get]
//	@Security		Bearer
func (atc *ApiTokenController) ListAllApiTokens(ctx *gin.Context) {
	authList := ctx.MustGet("authList").([]string)
	apiTokens, err := atc.ApiTokenService.ListAllApiTokens(authList)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(apiTokens)))
	if len(apiTokens) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, apiTokens)
}

// GetApiToken godoc
//
//	@Summary		Get a apiToken
//	@Description	Get information about a specific apiToken
//	@Tags			api-tokens
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"ApiToken ID"
//	@Success		200	{object}	models.ApiToken
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api-tokens/{id} [get]
//	@Security		Bearer
func (atc *ApiTokenController) GetApiToken(ctx *gin.Context) {
	apiTokenID := ctx.Param("id")

	_, err := uuid.FromString(apiTokenID)
	if err != nil {
		ctx.Error(errors.New("invalid api token id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	apiToken, err := atc.ApiTokenService.GetApiToken(apiTokenID)

	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, apiToken)
}

// CreateApiToken godoc
//
//	@Summary		Create a new apiToken
//	@Description	Add a apiToken to the cluster
//	@Tags			api-tokens
//	@Accept			json
//	@Produce		json
//	@Param			apiToken	body		models.ApiToken	true	"New apiToken"
//	@Success		200		{object}	models.ApiToken
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/api-tokens [post]
//	@Security		Bearer
func (atc *ApiTokenController) CreateApiToken(ctx *gin.Context) {
	var apiToken models.ApiToken

	if err := ctx.ShouldBindJSON(&apiToken); err != nil {
		ctx.Error(errors.New("invalid api token payload format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	apiToken, err := atc.ApiTokenService.CreateApiToken(getActor(ctx), apiToken)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, apiToken)
}

// UpdateApiToken godoc
//
//	@Summary		Update a apiToken
//	@Description	Update a apiToken in the cluster
//	@Tags			api-tokens
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string		true	"ApiToken ID"
//	@Param			apiToken	body		models.ApiToken	true	"Update apiToken"
//	@Success		200		{object}	models.ApiToken
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/api-tokens/{id} [patch]
//	@Security		Bearer
func (atc *ApiTokenController) UpdateApiToken(ctx *gin.Context) {
	apiTokenID := ctx.Param("id")
	var apiToken models.ApiToken
	var err error

	if err := ctx.ShouldBindJSON(&apiToken); err != nil {
		ctx.Error(errors.New("invalid api token payload format"))
		ctx.Status(http.StatusBadRequest)
		return
	}

	apiToken.ID, err = uuid.FromString(apiTokenID)
	if err != nil {
		helpers.BadRequestError(ctx, err)
		return
	}

	apiToken, err = atc.ApiTokenService.UpdateApiToken(getActor(ctx), apiToken)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, apiToken)
}

// DeleteApiToken godoc
//
//	@Summary		Delete a apiToken
//	@Description	Delete by apiToken ID
//	@Tags			api-tokens
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"ApiToken ID"
//	@Success		200	{object}	models.ResponseMessage
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/api-tokens/{id} [delete]
//	@Security		Bearer
func (atc *ApiTokenController) DeleteApiToken(ctx *gin.Context) {
	apiTokenID := ctx.Param("id")

	_, err := uuid.FromString(apiTokenID)
	if err != nil {
		ctx.Error(errors.New("invalid api token id format"))
		ctx.Status(http.StatusBadRequest)
		return
	}
	err = atc.ApiTokenService.DeleteApiToken(getActor(ctx), apiTokenID)
	if err != nil {
		ctx.Error(err)
		return
	}

	ctx.JSON(http.StatusOK, models.ResponseMessage{Message: "api token deleted successfully"})
}
