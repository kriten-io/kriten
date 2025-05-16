package middlewares

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/helpers"
	"github.com/kriten-io/kriten/models"
	"github.com/kriten-io/kriten/services"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

func AuthenticationMiddleware(as services.AuthService, jwtConf config.JWTConfig) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var token string
		webhookID := ctx.Param("id")
		// webhook-timestamp, webhook-id and webhook-signature - are specific header fields of Opsmil Infrahub webhook
		webhookTimestamp := ctx.GetHeader("webhook-timestamp")
		webhookMsgID := ctx.GetHeader("webhook-id")
		webhookSig := ctx.GetHeader("webhook-signature")
		// X-Hook-Signature header field is common webhook signature field, supported by Netbox and Nautobot
		signature := ctx.GetHeader("X-Hook-Signature")
		token = ctx.GetHeader("Token")
		if strings.Contains(ctx.Request.URL.String(), "/api/v1/webhooks/run") {
			if signature == "" && webhookSig == "" {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "webhook authentication failed."})
				return
			}

			body, err := io.ReadAll(ctx.Request.Body)

			if err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
				return
			}

			if webhookMsgID != "" && webhookTimestamp != "" && webhookSig != "" {
				owner, taskID, err := as.ValidateWebhookSignatureInfraHub(
					webhookID,
					webhookMsgID,
					webhookTimestamp,
					webhookSig,
					body)
				if err != nil {
					ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "webhook authentication failed."})
					return
				}
				ctx.Set("userID", owner.ID)
				ctx.Set("username", owner.Username)
				ctx.Set("provider", owner.Provider)
				ctx.Set("taskID", taskID)
			} else if signature != "" {
				owner, taskID, err := as.ValidateWebhookSignatureCommon(
					webhookID,
					signature,
					body)
				if err != nil {
					ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "webhook authentication failed."})
					return
				}
				ctx.Set("userID", owner.ID)
				ctx.Set("username", owner.Username)
				ctx.Set("provider", owner.Provider)
				ctx.Set("taskID", taskID)
			} else {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "webhook authentication failed."})
				return
			}

			ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		} else if token != "" {
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
