package controllers

import (
	"fmt"
	"kriten/config"
	"kriten/middlewares"
	"kriten/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AuditController struct {
	AuditService services.AuditService
	AuthService  services.AuthService
}

func NewAuditController(als services.AuditService, as services.AuthService) AuditController {
	return AuditController{
		AuditService: als,
		AuthService:  as,
	}
}

func (ac *AuditController) SetAuditRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.AuthenticationMiddleware(config.JWT))

	r.Use(middlewares.AuthorizationMiddleware(ac.AuthService, "audit", "read"))
	{
		r.GET("", ac.ListAudit)
		// r.GET("/:id", ac.GetAudit)
	}
}

// ListAudit godoc
//
//	@Summary		List audit
//	@Description	List all audit logs
//	@Tags			audit
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		models.AuditLog
//	@Failure		400	{object}	helpers.HTTPError
//	@Failure		404	{object}	helpers.HTTPError
//	@Failure		500	{object}	helpers.HTTPError
//	@Router			/groups [get]
//	@Security		Bearer
func (ac *AuditController) ListAudit(ctx *gin.Context) {
	var err error
	// Default limit
	max := 100
	param := ctx.Request.URL.Query().Get("max")

	if param != "" {
		max, err = strconv.Atoi(param)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	}

	groups, err := ac.AuditService.ListAudits(max)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-range", fmt.Sprintf("%v", len(groups)))
	if len(groups) == 0 {
		var arr [0]int
		ctx.JSON(http.StatusOK, arr)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.JSON(http.StatusOK, groups)
}
