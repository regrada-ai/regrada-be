package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/auth"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

const (
	idTokenCookie = "id_token"
)

type CookieAuthMiddleware struct {
	authService auth.Service
	userRepo    storage.UserRepository
	memberRepo  storage.OrganizationMemberRepository
}

func NewCookieAuthMiddleware(authService auth.Service, userRepo storage.UserRepository, memberRepo storage.OrganizationMemberRepository) *CookieAuthMiddleware {
	return &CookieAuthMiddleware{
		authService: authService,
		userRepo:    userRepo,
		memberRepo:  memberRepo,
	}
}

func (m *CookieAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get ID token from cookie (works for both native and federated users)
		idToken, err := c.Cookie(idTokenCookie)
		if err != nil || idToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Authentication required. Please log in.",
				},
			})
			c.Abort()
			return
		}

		// Validate ID token and extract user info
		userInfo, _, err := m.authService.ValidateIDToken(c.Request.Context(), idToken)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				c.Abort()
				return
			}
			log.Printf("Cookie auth validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "TOKEN_EXPIRED",
					"message": "Session expired. Please log in again.",
				},
			})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("sub", userInfo.Sub)
		c.Set("email", userInfo.Email)
		c.Set("name", userInfo.Name)

		// Look up user and organization from database
		user, err := m.userRepo.GetByIDPSub(c.Request.Context(), userInfo.Sub)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// User exists in Cognito but not in our database yet
				// This can happen for new OAuth users before they complete onboarding
				c.Set("user_id", "")
				c.Set("organization_id", "")
				c.Next()
				return
			}
			log.Printf("Failed to look up user by IDP sub: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to authenticate",
				},
			})
			c.Abort()
			return
		}

		c.Set("user_id", user.ID)

		// Look up organization membership (users belong to at most one org)
		memberships, err := m.memberRepo.ListByUser(c.Request.Context(), user.ID)
		if err != nil {
			log.Printf("Failed to look up user memberships: %v", err)
			c.Set("organization_id", "")
			c.Set("role", "")
		} else if len(memberships) == 1 {
			c.Set("organization_id", memberships[0].OrganizationID)
			c.Set("role", string(memberships[0].Role))
		} else {
			c.Set("organization_id", "")
			c.Set("role", "")
		}

		c.Next()
	}
}

// Optional middleware that checks for authentication but doesn't fail if not present
func (m *CookieAuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		idToken, err := c.Cookie(idTokenCookie)
		if err != nil || idToken == "" {
			c.Next()
			return
		}

		userInfo, _, err := m.authService.ValidateIDToken(c.Request.Context(), idToken)
		if err != nil {
			c.Next()
			return
		}

		// Store user information if available
		c.Set("sub", userInfo.Sub)
		c.Set("email", userInfo.Email)
		c.Set("name", userInfo.Name)

		// Look up user and organization from database
		user, err := m.userRepo.GetByIDPSub(c.Request.Context(), userInfo.Sub)
		if err != nil {
			c.Next()
			return
		}

		c.Set("user_id", user.ID)

		// Look up organization membership (users belong to at most one org)
		memberships, err := m.memberRepo.ListByUser(c.Request.Context(), user.ID)
		if err == nil && len(memberships) == 1 {
			c.Set("organization_id", memberships[0].OrganizationID)
			c.Set("role", string(memberships[0].Role))
		}

		c.Next()
	}
}
