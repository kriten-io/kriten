package middlewares

import (
	"bytes"
	"errors"
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
				owner, task, err := as.ValidateWebhookSignatureInfraHub(
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
				ctx.Set("task", task)
			} else if signature != "" {
				owner, task, err := as.ValidateWebhookSignatureCommon(
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
				ctx.Set("task", task)
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
		} else if bearer := bearerToken(ctx.GetHeader("Authorization")); bearer != "" {
			// A Bearer value is a JWT or a static API token.
			if strings.Count(bearer, ".") == 2 {
				if claims, err := helpers.ValidateJWTToken(bearer, jwtConf); err == nil {
					setAuthContext(ctx, claims.UserID, claims.Username, claims.Provider)
					ctx.Next()
					return
				}
			}
			owner, err := as.ValidateAPIToken(bearer)
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			setAuthContext(ctx, owner.ID, owner.Username, owner.Provider)
		} else {
			// Cookies carry a JWT only.
			cookie, err := ctx.Request.Cookie("token")
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please authenticate."})
				return
			}

			if strings.Count(cookie.Value, ".") == 2 {
				if claims, err := helpers.ValidateJWTToken(cookie.Value, jwtConf); err == nil {
					setAuthContext(ctx, claims.UserID, claims.Username, claims.Provider)
					ctx.Next()
					return
				}
			}
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token."})
			return
		}
		ctx.Next()
	}
}

// MCPAuthenticationMiddleware authenticates MCP requests from headers only
// (cookies are not accepted). It accepts a static API token in the Token header
// or in Authorization: Bearer, and a JWT in Authorization: Bearer. No
// WWW-Authenticate challenge is sent: Kriten does not implement OAuth, and a
// Bearer challenge makes clients start OAuth discovery.
func MCPAuthenticationMiddleware(as services.AuthService, jwtConf config.JWTConfig) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := strings.TrimSpace(ctx.GetHeader("Token"))
		bearer := bearerToken(ctx.GetHeader("Authorization"))
		if token == "" && bearer == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authenticate with a Token header or Authorization: Bearer <api token>."})
			return
		}

		if token != "" {
			owner, err := as.ValidateAPIToken(token)
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			setAuthContext(ctx, owner.ID, owner.Username, owner.Provider)
			ctx.Next()
			return
		}

		// API tokens are opaque keys, not JWTs; only JWT-shaped values are parsed
		// so that API token requests don't log JWT parse errors.
		if strings.Count(bearer, ".") == 2 {
			if claims, err := helpers.ValidateJWTToken(bearer, jwtConf); err == nil {
				setAuthContext(ctx, claims.UserID, claims.Username, claims.Provider)
				ctx.Next()
				return
			}
		}

		owner, err := as.ValidateAPIToken(bearer)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		setAuthContext(ctx, owner.ID, owner.Username, owner.Provider)
		ctx.Next()
	}
}

// bearerToken returns the credential from an Authorization header with the
// "Bearer" scheme. The scheme is matched case-insensitively, as RFC 7235 requires.
func bearerToken(header string) string {
	scheme, value, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}

func setAuthContext(ctx *gin.Context, userID uuid.UUID, username string, provider string) {
	ctx.Set("userID", userID)
	ctx.Set("username", username)
	ctx.Set("provider", provider)
}

func AuthorizationMiddleware(as services.AuthService, resource string, access string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID := ctx.MustGet("userID").(uuid.UUID)
		provider := ctx.MustGet("provider").(string)

		var task string
		if value, ok := ctx.Get("task"); ok {
			switch t := value.(type) {
			case string:
				task = t
			case uuid.UUID:
				task = t.String()
			}
		}

		resourceName := ""
		if resource == "jobs" && access == "read" {
			jobID, err := uuid.FromString(ctx.Param("id"))
			if err != nil {
				ctx.Error(errors.New("invalid job id format"))
				ctx.AbortWithStatus(http.StatusBadRequest)
				return
			}

			taskID, err := as.GetJobTaskID(jobID)
			if err != nil {
				if errors.Is(err, services.ErrSvcObjNotFound) {
					ctx.Error(err)
					ctx.AbortWithStatus(http.StatusNotFound)
					return
				}
				log.Println(err)
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error."})
				return
			}

			resourceName = taskID.String()
		} else {
			if ctx.Param("name") != "" {
				resourceName = ctx.Param("name")
			} else if task != "" {
				resourceName = task
			} else if ctx.Param("id") != "" {
				resourceName = ctx.Param("id")
			}

			if resourceName == "" {
				resourceName = "*"
			}
		}

		isAuthorised, err := as.IsAutorised(
			&models.Authorization{
				UserID:       userID,
				Provider:     provider,
				Resource:     resource,
				ResourceName: resourceName,
				Access:       access,
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
