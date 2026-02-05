package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type CognitoMiddleware struct {
	jwkSet    jwk.Set
	region    string
	userPoolID string
	cache     *jwk.Cache
}

func NewCognitoMiddleware(region, userPoolID string) (*CognitoMiddleware, error) {
	if region == "" || userPoolID == "" {
		return nil, fmt.Errorf("AWS region and user pool ID are required")
	}

	// Construct the JWKS URL for Cognito
	jwksURL := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", region, userPoolID)

	// Create a cache for JWKS
	cache := jwk.NewCache(context.Background())

	// Register the JWKS URL with the cache
	if err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, fmt.Errorf("failed to register JWKS URL: %w", err)
	}

	// Fetch the initial keyset
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		log.Printf("Warning: failed to fetch initial JWKS: %v", err)
	}

	log.Printf("✓ Cognito JWT middleware initialized for region: %s, pool: %s", region, userPoolID)

	return &CognitoMiddleware{
		region:     region,
		userPoolID: userPoolID,
		cache:      cache,
	}, nil
}

func (m *CognitoMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		token := extractBearerToken(c.Request)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Missing or invalid authorization header",
				},
			})
			c.Abort()
			return
		}

		// Get JWKS
		jwksURL := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", m.region, m.userPoolID)
		keyset, err := m.cache.Get(c.Request.Context(), jwksURL)
		if err != nil {
			log.Printf("Failed to get JWKS: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to validate token",
				},
			})
			c.Abort()
			return
		}

		// Parse and validate the token
		parsedToken, err := jwt.Parse(
			[]byte(token),
			jwt.WithKeySet(keyset),
			jwt.WithValidate(true),
		)

		if err != nil {
			log.Printf("JWT validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid token",
				},
			})
			c.Abort()
			return
		}

		// Extract claims
		sub, ok := parsedToken.Get("sub")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid token claims",
				},
			})
			c.Abort()
			return
		}

		email := ""
		if emailClaim, exists := parsedToken.Get("email"); exists {
			if emailStr, ok := emailClaim.(string); ok {
				email = emailStr
			}
		}

		name := ""
		if nameClaim, exists := parsedToken.Get("name"); exists {
			if nameStr, ok := nameClaim.(string); ok {
				name = nameStr
			}
		}

		// Store user information in context
		c.Set("user_id", sub)
		c.Set("email", email)
		c.Set("name", name)
		// Note: organization_id is looked up from database, not from JWT claims

		// Store full token for debugging if needed
		c.Set("jwt_token", parsedToken)

		c.Next()
	}
}

// extractBearerToken extracts the JWT token from the Authorization header
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Expected format: "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// GetUserIDFromContext is a helper to extract user_id from gin.Context
func GetUserIDFromContext(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", errors.New("user_id not found in context")
	}

	userIDStr, ok := userID.(string)
	if !ok {
		return "", errors.New("user_id is not a string")
	}

	return userIDStr, nil
}

// GetOrganizationIDFromContext is a helper to extract organization_id from gin.Context
func GetOrganizationIDFromContext(c *gin.Context) (string, error) {
	orgID, exists := c.Get("organization_id")
	if !exists || orgID == "" {
		return "", errors.New("organization_id not found in context")
	}

	orgIDStr, ok := orgID.(string)
	if !ok {
		return "", errors.New("organization_id is not a string")
	}

	if orgIDStr == "" {
		return "", errors.New("organization_id is empty")
	}

	return orgIDStr, nil
}
