package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/auth"
)

const (
	accessTokenCookie  = "access_token"
	idTokenCookie      = "id_token"
	refreshTokenCookie = "refresh_token"
	cookieMaxAge       = 3600 // 1 hour
	cookiePath         = "/"
	cookieDomain       = "" // Empty for localhost, set for production
)

type AuthHandler struct {
	cognitoService *auth.CognitoService
	secureCookies  bool
}

func NewAuthHandler(cognitoService *auth.CognitoService, secureCookies bool) *AuthHandler {
	return &AuthHandler{
		cognitoService: cognitoService,
		secureCookies:  secureCookies,
	}
}

// SignUp handles user registration
func (h *AuthHandler) SignUp(c *gin.Context) {
	var req struct {
		Email          string `json:"email" binding:"required,email"`
		Password       string `json:"password" binding:"required,min=8"`
		Name           string `json:"name" binding:"required"`
		OrganizationID string `json:"organization_id,omitempty"`
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

	result, err := h.cognitoService.SignUp(c.Request.Context(), req.Email, req.Password, req.Name, req.OrganizationID)
	if err != nil {
		log.Printf("SignUp error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "SIGNUP_FAILED",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":        true,
		"user_confirmed": result.UserConfirmed,
		"message":        "Sign up successful. Please check your email for verification code.",
	})
}

// ConfirmSignUp handles email verification
func (h *AuthHandler) ConfirmSignUp(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
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

	if err := h.cognitoService.ConfirmSignUp(c.Request.Context(), req.Email, req.Code); err != nil {
		log.Printf("ConfirmSignUp error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VERIFICATION_FAILED",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Email verified successfully. You can now sign in.",
	})
}

// SignIn handles user login
func (h *AuthHandler) SignIn(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
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

	tokens, err := h.cognitoService.SignIn(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("SignIn error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "SIGNIN_FAILED",
				"message": "Invalid email or password",
			},
		})
		return
	}

	// Set HTTP-only cookies for tokens
	h.setAuthCookies(c, tokens)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Sign in successful",
	})
}

// SignOut handles user logout
func (h *AuthHandler) SignOut(c *gin.Context) {
	// Get access token from cookie
	accessToken, err := c.Cookie(accessTokenCookie)
	if err == nil && accessToken != "" {
		// Try to sign out from Cognito (best effort)
		if err := h.cognitoService.SignOut(c.Request.Context(), accessToken); err != nil {
			log.Printf("Cognito SignOut error (non-fatal): %v", err)
		}
	}

	// Clear cookies regardless of Cognito signout result
	h.clearAuthCookies(c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Signed out successfully",
	})
}

// Me returns the current user's information
func (h *AuthHandler) Me(c *gin.Context) {
	// Get access token from cookie
	accessToken, err := c.Cookie(accessTokenCookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Not authenticated",
			},
		})
		return
	}

	userInfo, err := h.cognitoService.GetUser(c.Request.Context(), accessToken)
	if err != nil {
		log.Printf("GetUser error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Invalid or expired session",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"sub":             userInfo.Sub,
			"email":           userInfo.Email,
			"name":            userInfo.Name,
			"picture":         userInfo.Picture,
			"organization_id": userInfo.OrganizationID,
		},
	})
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// Get refresh token from cookie
	refreshToken, err := c.Cookie(refreshTokenCookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "No refresh token found",
			},
		})
		return
	}

	tokens, err := h.cognitoService.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		log.Printf("RefreshToken error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "REFRESH_FAILED",
				"message": "Failed to refresh token",
			},
		})
		return
	}

	// Update access and ID tokens (refresh token stays the same)
	h.setAuthCookies(c, tokens)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token refreshed successfully",
	})
}

// setAuthCookies sets the authentication cookies
func (h *AuthHandler) setAuthCookies(c *gin.Context, tokens *auth.AuthTokens) {
	sameSite := http.SameSiteLaxMode
	if h.secureCookies {
		sameSite = http.SameSiteNoneMode
	}

	// Access token cookie
	c.SetCookie(
		accessTokenCookie,
		tokens.AccessToken,
		cookieMaxAge,
		cookiePath,
		cookieDomain,
		h.secureCookies, // Secure flag (HTTPS only)
		true,            // HTTP-only flag
	)
	c.SetSameSite(sameSite)

	// ID token cookie
	c.SetCookie(
		idTokenCookie,
		tokens.IDToken,
		cookieMaxAge,
		cookiePath,
		cookieDomain,
		h.secureCookies,
		true,
	)

	// Refresh token cookie (longer expiry)
	if tokens.RefreshToken != "" {
		c.SetCookie(
			refreshTokenCookie,
			tokens.RefreshToken,
			30*24*3600, // 30 days
			cookiePath,
			cookieDomain,
			h.secureCookies,
			true,
		)
	}
}

// clearAuthCookies removes all authentication cookies
func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	cookies := []string{accessTokenCookie, idTokenCookie, refreshTokenCookie}

	for _, cookie := range cookies {
		c.SetCookie(
			cookie,
			"",
			-1, // MaxAge -1 deletes the cookie
			cookiePath,
			cookieDomain,
			h.secureCookies,
			true,
		)
	}
}
