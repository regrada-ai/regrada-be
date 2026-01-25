package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/auth"
)

const (
	accessTokenCookie = "access_token"
)

type CookieAuthMiddleware struct {
	cognitoService *auth.CognitoService
}

func NewCookieAuthMiddleware(cognitoService *auth.CognitoService) *CookieAuthMiddleware {
	return &CookieAuthMiddleware{
		cognitoService: cognitoService,
	}
}

func (m *CookieAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get access token from cookie
		accessToken, err := c.Cookie(accessTokenCookie)
		if err != nil || accessToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Authentication required. Please log in.",
				},
			})
			c.Abort()
			return
		}

		// Get user info from Cognito using the access token
		userInfo, err := m.cognitoService.GetUser(c.Request.Context(), accessToken)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				c.Abort()
				return
			}
			log.Printf("Cookie auth validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid or expired session. Please log in again.",
				},
			})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user_id", userInfo.Sub)
		c.Set("email", userInfo.Email)
		c.Set("name", userInfo.Name)
		c.Set("organization_id", userInfo.OrganizationID)

		c.Next()
	}
}

// Optional middleware that checks for authentication but doesn't fail if not present
func (m *CookieAuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		accessToken, err := c.Cookie(accessTokenCookie)
		if err != nil || accessToken == "" {
			c.Next()
			return
		}

		userInfo, err := m.cognitoService.GetUser(c.Request.Context(), accessToken)
		if err != nil {
			c.Next()
			return
		}

		// Store user information if available
		c.Set("user_id", userInfo.Sub)
		c.Set("email", userInfo.Email)
		c.Set("name", userInfo.Name)
		c.Set("organization_id", userInfo.OrganizationID)

		c.Next()
	}
}
