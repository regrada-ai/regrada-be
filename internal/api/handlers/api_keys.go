package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

var defaultAPIKeyScopes = []string{"traces:write", "tests:write", "projects:read"}

type APIKeyHandler struct {
	apiKeyRepo storage.APIKeyRepository
	orgRepo    storage.OrganizationRepository
}

func NewAPIKeyHandler(apiKeyRepo storage.APIKeyRepository, orgRepo storage.OrganizationRepository) *APIKeyHandler {
	return &APIKeyHandler{apiKeyRepo: apiKeyRepo, orgRepo: orgRepo}
}

type apiKeyResponse struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	KeyPrefix    string     `json:"key_prefix"`
	Tier         string     `json:"tier"`
	Scopes       []string   `json:"scopes"`
	RateLimitRPM int        `json:"rate_limit_rpm"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

func toAPIKeyResponse(key *storage.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID:           key.ID,
		Name:         key.Name,
		KeyPrefix:    key.KeyPrefix,
		Tier:         key.Tier,
		Scopes:       key.Scopes,
		RateLimitRPM: key.RateLimitRPM,
		LastUsedAt:   key.LastUsedAt,
		ExpiresAt:    key.ExpiresAt,
		CreatedAt:    key.CreatedAt,
		RevokedAt:    key.RevokedAt,
	}
}

// ListAPIKeys returns API keys for the authenticated organization.
// @Summary List API keys
// @Description Get all API keys for your organization
// @Tags api-keys
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Success 200 {object} object{api_keys=[]types.APIKeyResponse,count=int}
// @Failure 401 {object} types.ErrorResponse
// @Router /api-keys [get]
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	keys, err := h.apiKeyRepo.ListByOrganization(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch API keys",
			},
		})
		return
	}

	responses := make([]apiKeyResponse, len(keys))
	for i, key := range keys {
		responses[i] = toAPIKeyResponse(key)
	}

	c.JSON(http.StatusOK, gin.H{
		"api_keys": responses,
		"count":    len(responses),
	})
}

// CreateAPIKey creates a new API key for the authenticated organization.
// @Summary Create a new API key
// @Description Create a new API key for your organization. The secret is only returned once. The API key tier is inherited from the organization's tier.
// @Tags api-keys
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security CookieAuth
// @Param request body types.CreateAPIKeyRequest true "API key details"
// @Success 201 {object} types.CreateAPIKeyResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 401 {object} types.ErrorResponse
// @Router /api-keys [post]
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	var req struct {
		Name      string     `json:"name" binding:"required"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[CreateAPIKey] binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request parameters",
			},
		})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "name is required",
			},
		})
		return
	}

	// Get organization to determine tier
	org, err := h.orgRepo.Get(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch organization",
			},
		})
		return
	}

	tier := org.Tier
	rateLimitRPM := rateLimitForTier(tier)

	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = defaultAPIKeyScopes
	}

	secret, keyHash, keyPrefix, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to generate API key",
			},
		})
		return
	}

	apiKey := &storage.APIKey{
		OrganizationID: orgID,
		KeyHash:        keyHash,
		KeyPrefix:      keyPrefix,
		Name:           name,
		Tier:           tier,
		Scopes:         scopes,
		RateLimitRPM:   rateLimitRPM,
		ExpiresAt:      req.ExpiresAt,
	}

	if err := h.apiKeyRepo.Create(c.Request.Context(), apiKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create API key",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"api_key": toAPIKeyResponse(apiKey),
		"secret":  secret,
	})
}

func generateAPIKey() (secret, hash, prefix string, err error) {
	randomBytes := make([]byte, 32)
	if _, err = rand.Read(randomBytes); err != nil {
		return "", "", "", err
	}

	secret = "rg_live_" + base64.RawURLEncoding.EncodeToString(randomBytes)
	checksum := sha256.Sum256([]byte(secret))
	hash = hex.EncodeToString(checksum[:])
	if len(secret) >= 16 {
		prefix = secret[:16]
	} else {
		prefix = secret
	}
	return secret, hash, prefix, nil
}

func rateLimitForTier(tier string) int {
	switch tier {
	case "starter":
		return 10
	case "team":
		return 100
	case "scale":
		return 500
	case "enterprise":
		return 2000
	default:
		return 10 // Default to starter tier rate limit
	}
}

func monthlyLimitForTier(tier string) int64 {
	switch tier {
	case "starter":
		return 50_000
	case "team":
		return 500_000
	case "scale":
		return 5_000_000
	case "enterprise":
		return 20_000_000
	default:
		return 50_000 // Default to starter tier limit
	}
}

// GetAPIKey retrieves a single API key by ID
func (h *APIKeyHandler) GetAPIKey(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	keyID := c.Param("keyID")

	key, err := h.apiKeyRepo.GetByID(c.Request.Context(), keyID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "API key not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch API key",
			},
		})
		return
	}

	// Verify the key belongs to the requesting organization
	if key.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot access API keys from a different organization",
			},
		})
		return
	}

	c.JSON(http.StatusOK, toAPIKeyResponse(key))
}

// UpdateAPIKey updates an existing API key
func (h *APIKeyHandler) UpdateAPIKey(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	keyID := c.Param("keyID")

	var req struct {
		Name      *string    `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[UpdateAPIKey] binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request parameters",
			},
		})
		return
	}

	key, err := h.apiKeyRepo.GetByID(c.Request.Context(), keyID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "API key not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch API key",
			},
		})
		return
	}

	// Verify the key belongs to the requesting organization
	if key.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot update API keys from a different organization",
			},
		})
		return
	}

	// Update fields (tier is not updatable - it's linked to organization tier)
	if req.Name != nil {
		key.Name = *req.Name
	}
	if req.Scopes != nil {
		key.Scopes = req.Scopes
	}
	if req.ExpiresAt != nil {
		key.ExpiresAt = req.ExpiresAt
	}

	if err := h.apiKeyRepo.Update(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to update API key",
			},
		})
		return
	}

	c.JSON(http.StatusOK, toAPIKeyResponse(key))
}

// RevokeAPIKey revokes an API key
func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	keyID := c.Param("keyID")

	key, err := h.apiKeyRepo.GetByID(c.Request.Context(), keyID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "API key not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch API key",
			},
		})
		return
	}

	// Verify the key belongs to the requesting organization
	if key.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot revoke API keys from a different organization",
			},
		})
		return
	}

	if err := h.apiKeyRepo.Revoke(c.Request.Context(), keyID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "API key not found or already revoked",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to revoke API key",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// DeleteAPIKey permanently deletes an API key
func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	keyID := c.Param("keyID")

	key, err := h.apiKeyRepo.GetByID(c.Request.Context(), keyID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "API key not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch API key",
			},
		})
		return
	}

	// Verify the key belongs to the requesting organization
	if key.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot delete API keys from a different organization",
			},
		})
		return
	}

	if err := h.apiKeyRepo.Delete(c.Request.Context(), keyID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "API key not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to delete API key",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
