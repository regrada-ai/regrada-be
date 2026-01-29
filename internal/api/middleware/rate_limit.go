// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimitMiddleware struct {
	redisClient *redis.Client
}

func NewRateLimitMiddleware(redisClient *redis.Client) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		redisClient: redisClient,
	}
}

func (m *RateLimitMiddleware) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		keyHash, exists := c.Get("api_key_hash")
		if !exists {
			// Auth middleware should have set this
			c.Next()
			return
		}

		tier, _ := c.Get("tier")
		limit := m.getLimitForTier(tier.(string))

		ctx := c.Request.Context()
		now := time.Now()
		window := now.Unix() / 60 // 1-minute window

		rateLimitKey := fmt.Sprintf("ratelimit:%s:%d", keyHash, window)

		// Increment counter
		count, err := m.redisClient.Incr(ctx, rateLimitKey).Result()
		if err != nil {
			// On Redis error, allow the request (fail open)
			c.Next()
			return
		}

		// Set expiry on first increment
		if count == 1 {
			m.redisClient.Expire(ctx, rateLimitKey, 2*time.Minute)
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, limit-int(count))))
		c.Header("X-RateLimit-Reset", strconv.FormatInt((window+1)*60, 10))

		// Check if over limit
		if count > int64(limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Rate limit exceeded",
					"details": gin.H{
						"limit": limit,
						"reset": (window + 1) * 60,
					},
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (m *RateLimitMiddleware) getLimitForTier(tier string) int {
	switch tier {
	case "enterprise":
		return 2000 // 2000 RPM
	case "pro":
		return 500 // 500 RPM
	case "standard":
		return 100 // 100 RPM
	default:
		return 100
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
