package controllers

import (
	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/middlewares"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
)

type McpController struct {
	McpService  services.McpService
	AuthService services.AuthService
}

func NewMcpController(ms services.McpService, as services.AuthService) McpController {
	return McpController{
		McpService:  ms,
		AuthService: as,
	}
}

// The MCP transport handles GET, POST and DELETE itself, so every method is
// routed to the same handler.
func (mc *McpController) SetMcpRoutes(rg *gin.RouterGroup, config config.Config) {
	r := rg.Group("").Use(
		middlewares.HeaderAuthOnlyMiddleware(),
		middlewares.AuthenticationMiddleware(mc.AuthService, config.JWT))

	r.Any("", mc.Serve)
}

func (mc *McpController) Serve(ctx *gin.Context) {
	req := ctx.Request.WithContext(services.WithCaller(ctx.Request.Context(), getAuditMetadata(ctx)))
	mc.McpService.Handler().ServeHTTP(ctx.Writer, req)
}
