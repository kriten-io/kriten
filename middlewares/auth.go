package middlewares

import (
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"kriten/services"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

func AuthenticationMiddleware(as services.AuthService, jwtConf config.JWTConfig) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var token string
		token = ctx.GetHeader("Token")
		if token != "" {
			owner, err := as.ValidateAPIToken(token)
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			ctx.Set("userID", owner.ID)
			ctx.Set("username", owner.Username)
			ctx.Set("provider", owner.Provider)
		} else {
			// If no API token is provided, checking for Bearer or Cookies
			bearer := strings.Split(ctx.GetHeader("Authorization"), "Bearer ")
			if len(bearer) > 1 {
				token = bearer[1]
			}
			cookie, err := ctx.Request.Cookie("token")
			if token == "" {
				if err != nil {
					ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please authenticate."})
					return
				}
				token = cookie.Value
			}

			claims, err := helpers.ValidateJWTToken(token, jwtConf)
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token."})
				return
			}

			ctx.Set("userID", claims.UserID)
			ctx.Set("username", claims.Username)
			ctx.Set("provider", claims.Provider)
		}
		ctx.Next()
	}
}

func AuthorizationMiddleware(as services.AuthService, resource string, access string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID := ctx.MustGet("userID").(uuid.UUID)
		provider := ctx.MustGet("provider").(string)
		requestUrl := ctx.Request.URL.String()

		resourceID := ctx.Param("id")
		if resourceID == "" {
			resourceID = "*"
		}

		// trimming last 6 chars for jobs read because
		// jobs include random caracters at the end
		if resource == "jobs" && access == "read" && !strings.HasSuffix(requestUrl, "/schema") {
			resourceID = resourceID[:len(resourceID)-6]
		}

		isAuthorised, err := as.IsAutorised(
			&models.Authorization{
				UserID:     userID,
				Provider:   provider,
				Resource:   resource,
				ResourceID: resourceID,
				Access:     access,
			},
		)
		if err != nil {
			log.Println(err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error."})
			return
		}

		if isAuthorised {
			ctx.Next()
			return
		}

		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unauthorized - user cannot access resource"})
	}
}

func SetAuthorizationListMiddleware(as services.AuthService, resource string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID := ctx.MustGet("userID").(uuid.UUID)
		provider := ctx.MustGet("provider").(string)

		authList, err := as.GetAuthorizationList(
			&models.Authorization{
				UserID:   userID,
				Provider: provider,
				Resource: resource,
			},
		)
		if err != nil {
			log.Println(err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error."})
			return
		}

		ctx.Set("authList", authList)
		ctx.Next()
	}
}
