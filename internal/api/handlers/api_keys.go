package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

var defaultAPIKeyScopes = []string{"traces:write", "tests:write", "projects:read"}

type APIKeyHandler struct {
	apiKeyRepo storage.APIKeyRepository
}

func NewAPIKeyHandler(apiKeyRepo storage.APIKeyRepository) *APIKeyHandler {
	return &APIKeyHandler{apiKeyRepo: apiKeyRepo}
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
		Tier      string     `json:"tier" binding:"required"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
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

	tier := strings.ToLower(strings.TrimSpace(req.Tier))
	rateLimitRPM, ok := rateLimitForTier(tier)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "tier must be standard, pro, or enterprise",
			},
		})
		return
	}

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

func rateLimitForTier(tier string) (int, bool) {
	switch tier {
	case "standard":
		return 100, true
	case "pro":
		return 500, true
	case "enterprise":
		return 2000, true
	default:
		return 0, false
	}
}
