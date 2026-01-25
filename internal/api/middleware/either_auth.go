package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type EitherAuthMiddleware struct {
	cookieAuth *CookieAuthMiddleware
	apiKeyAuth *AuthMiddleware
}

func NewEitherAuthMiddleware(cookieAuth *CookieAuthMiddleware, apiKeyAuth *AuthMiddleware) *EitherAuthMiddleware {
	return &EitherAuthMiddleware{
		cookieAuth: cookieAuth,
		apiKeyAuth: apiKeyAuth,
	}
}

func (m *EitherAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.apiKeyAuth != nil && hasAuthorizationHeader(c) {
			m.apiKeyAuth.Authenticate()(c)
			return
		}

		if m.cookieAuth != nil {
			accessToken, err := c.Cookie(accessTokenCookie)
			if err == nil && accessToken != "" {
				m.cookieAuth.Authenticate()(c)
				return
			}
		}

		if m.apiKeyAuth != nil {
			m.apiKeyAuth.Authenticate()(c)
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Authentication required",
			},
		})
		c.Abort()
	}
}

func hasAuthorizationHeader(c *gin.Context) bool {
	return strings.TrimSpace(c.GetHeader("Authorization")) != ""
}
