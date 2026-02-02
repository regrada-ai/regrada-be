// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type AuthMiddleware struct {
	apiKeyRepo  storage.APIKeyRepository
	redisClient *redis.Client
}

func NewAuthMiddleware(apiKeyRepo storage.APIKeyRepository, redisClient *redis.Client) *AuthMiddleware {
	return &AuthMiddleware{
		apiKeyRepo:  apiKeyRepo,
		redisClient: redisClient,
	}
}

func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract API key from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Missing authorization header",
				},
			})
			c.Abort()
			return
		}

		// Expected format: "Bearer rg_live_..."
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid authorization header format",
				},
			})
			c.Abort()
			return
		}

		apiKey := parts[1]

		// Hash the API key
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(hash[:])

		// Check cache first
		ctx := c.Request.Context()
		cacheKey := "apikey:" + keyHash
		tierCacheKey := "apikey:tier:" + keyHash
		orgCacheKey := "apikey:org:" + keyHash

		// Try to get from cache
		cached, err := m.redisClient.Get(ctx, cacheKey).Result()
		cachedTier, tierErr := m.redisClient.Get(ctx, tierCacheKey).Result()
		cachedOrg, orgErr := m.redisClient.Get(ctx, orgCacheKey).Result()
		if err == nil && cached != "" && tierErr == nil && cachedTier != "" && orgErr == nil && cachedOrg != "" {
			// Key exists in cache with org/tier info, it's valid
			c.Set("api_key_hash", keyHash)
			c.Set("organization_id", cachedOrg)
			c.Set("tier", cachedTier)
			c.Next()
			return
		}

		// Not in cache, check database
		apiKeyData, err := m.apiKeyRepo.GetByHash(ctx, keyHash)
		if err != nil {
			if err == storage.ErrNotFound {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "UNAUTHORIZED",
						"message": "Invalid API key",
					},
				})
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to validate API key",
				},
			})
			c.Abort()
			return
		}

		// Check if key is revoked or expired
		if apiKeyData.RevokedAt != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "API key has been revoked",
				},
			})
			c.Abort()
			return
		}

		if apiKeyData.ExpiresAt != nil && apiKeyData.ExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "API key has expired",
				},
			})
			c.Abort()
			return
		}

		// Cache the valid key and tier for 5 minutes
		m.redisClient.Set(ctx, cacheKey, "valid", 5*time.Minute)
		m.redisClient.Set(ctx, tierCacheKey, apiKeyData.Tier, 5*time.Minute)
		m.redisClient.Set(ctx, orgCacheKey, apiKeyData.OrganizationID, 5*time.Minute)

		// Store in context
		c.Set("api_key_hash", keyHash)
		c.Set("organization_id", apiKeyData.OrganizationID)
		c.Set("tier", apiKeyData.Tier)

		// Update last used timestamp (async, don't block request)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			m.apiKeyRepo.UpdateLastUsed(ctx, apiKeyData.ID)
		}()

		c.Next()
	}
}
