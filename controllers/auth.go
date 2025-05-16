package controllers

import (
	"fmt"
	"net/http"

	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slices"
)

type AuthController struct {
	AuthService   services.AuthService
	providers     []string
	AuditService  services.AuditService
	AuditCategory string
}

func NewAuthController(as services.AuthService, als services.AuditService, p []string) AuthController {
	return AuthController{
		AuthService:   as,
		AuditService:  als,
		AuditCategory: "authentication",
		providers:     p,
	}
}

func (ac *AuthController) SetAuthRoutes(rg *gin.RouterGroup) {
	rg.POST("/login", ac.Login)
	rg.GET("/refresh", ac.Refresh)
}

// Login godoc
//
//	@Summary		Authenticate users
//	@Description	authenticate and generates a JWT token
//	@Tags			authenticate
//	@Accept			json
//	@Produce		json
//	@Param			credentials	body		models.Credentials	true	"Your credentials"
//	@Success		200			{object}	string
//	@Failure		400			{object}	helpers.HTTPError
//	@Failure		401			{object}	helpers.HTTPError
//	@Failure		404			{object}	helpers.HTTPError
//	@Failure		500			{object}	helpers.HTTPError
//	@Router			/login [post]
func (ac *AuthController) Login(ctx *gin.Context) {
	// timestamp := time.Now().UTC()
	var credentials models.Credentials
	audit := ac.AuditService.InitialiseAuditLog(ctx, "login", ac.AuditCategory, "*")

	if err := ctx.ShouldBindJSON(&credentials); err != nil {
		ac.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	audit.UserName = credentials.Username
	audit.Provider = credentials.Provider

	if !slices.Contains(ac.providers, credentials.Provider) {
		ac.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider does not exist", "providers": ac.providers})
		return
	}

	token, expiry, err := ac.AuthService.Login(&credentials)
	if err != nil {
		fmt.Println("error:", err)
		ac.AuditService.CreateAudit(audit)
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials."})
		return
	}

	audit.Status = "success"
	ac.AuditService.CreateAudit(audit)

	ctx.SetSameSite(http.SameSiteNoneMode)
	ctx.SetCookie("token", token, expiry, "", "", false, true)
	ctx.JSON(http.StatusOK, gin.H{"token": token})
}

// Refresh godoc
//
//	@Summary		Auth admin
//	@Description	Refresh time limit of a JWT token
//	@Tags			authenticate
//	@Accept			json
//	@Produce		json
//	@Param			token	header		string	false	"JWT Token can be provided as Cookie"
//	@Success		200		{object}	string
//	@Failure		400		{object}	helpers.HTTPError
//	@Failure		401		{object}	helpers.HTTPError
//	@Failure		404		{object}	helpers.HTTPError
//	@Failure		500		{object}	helpers.HTTPError
//	@Router			/refresh [get]
//	@Security		Bearer
func (ac *AuthController) Refresh(ctx *gin.Context) {
	token, err := ctx.Request.Cookie("token")
	if err == http.ErrNoCookie {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "please authenticate."})
		return
	}

	newToken, expiry, err := ac.AuthService.Refresh(token.Value)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token."})
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("token", newToken, expiry, "", "", false, true)
	ctx.JSON(http.StatusOK, gin.H{"token": newToken})
}
