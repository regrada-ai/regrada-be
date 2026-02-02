package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// NewCORSMiddleware allows browser clients to call the API directly.
func NewCORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		value := strings.TrimSpace(origin)
		if value == "" {
			continue
		}
		if value == "*" {
			allowAll = true
		}
		allowed[value] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				// Allow credentials (cookies) only for specific origins, not wildcard
				c.Header("Access-Control-Allow-Credentials", "true")
			}

			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
			c.Header("Access-Control-Max-Age", "7200")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
