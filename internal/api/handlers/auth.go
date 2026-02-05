package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/auth"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

const (
	accessTokenCookie  = "access_token"
	idTokenCookie      = "id_token"
	refreshTokenCookie = "refresh_token"
	cookieMaxAge       = 3600 // 1 hour
	cookiePath         = "/"
)

type AuthHandler struct {
	authService    auth.Service
	secureCookies  bool
	cookieDomain   string
	userRepo       storage.UserRepository
	memberRepo     storage.OrganizationMemberRepository
	orgRepo        storage.OrganizationRepository
	inviteRepo     storage.InviteRepository
	storageService storage.FileStorageService
}

func NewAuthHandler(
	authService auth.Service,
	secureCookies bool,
	cookieDomain string,
	userRepo storage.UserRepository,
	memberRepo storage.OrganizationMemberRepository,
	orgRepo storage.OrganizationRepository,
	inviteRepo storage.InviteRepository,
	storageService storage.FileStorageService,
) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		secureCookies:  secureCookies,
		cookieDomain:   cookieDomain,
		userRepo:       userRepo,
		memberRepo:     memberRepo,
		orgRepo:        orgRepo,
		inviteRepo:     inviteRepo,
		storageService: storageService,
	}
}

// SignUp handles user registration
// @Summary Register a new user
// @Description Register a new user account with optional organization creation or invite token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object{email=string,password=string,name=string,create_organization=bool,organization_name=string,invite_token=string} true "Signup request"
// @Success 201 {object} object{success=bool,user_confirmed=bool,organization_id=string,message=string}
// @Failure 400 {object} object{error=object{code=string,message=string}}
// @Router /auth/signup [post]
func (h *AuthHandler) SignUp(c *gin.Context) {
	var req struct {
		Email              string `json:"email" binding:"required,email"`
		Password           string `json:"password" binding:"required,min=8"`
		Name               string `json:"name" binding:"required"`
		CreateOrganization bool   `json:"create_organization,omitempty"`
		OrganizationName   string `json:"organization_name,omitempty"`
		InviteToken        string `json:"invite_token,omitempty"`
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

	var orgID string
	var dbToken string // The raw token for DB lookups

	// URL decode the invite token if present (in case it was encoded in the URL)
	inviteToken := req.InviteToken
	if inviteToken != "" {
		if decoded, err := url.QueryUnescape(inviteToken); err == nil {
			inviteToken = decoded
		}
		// Decode compound token to extract email and raw token
		_, dbToken, _ = decodeInviteToken(inviteToken)
	}

	// Handle invite token
	if inviteToken != "" {
		invite, err := h.inviteRepo.GetByToken(c.Request.Context(), dbToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVALID_INVITE",
					"message": "Invalid or expired invite token",
				},
			})
			return
		}

		if invite.AcceptedAt != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVITE_ALREADY_USED",
					"message": "This invite has already been accepted",
				},
			})
			return
		}

		if invite.RevokedAt != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVITE_REVOKED",
					"message": "This invite has been revoked",
				},
			})
			return
		}

		if time.Now().After(invite.ExpiresAt) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVITE_EXPIRED",
					"message": "This invite has expired",
				},
			})
			return
		}

		if !strings.EqualFold(invite.Email, req.Email) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "EMAIL_MISMATCH",
					"message": "Email does not match the invite",
				},
			})
			return
		}

		orgID = invite.OrganizationID
	} else if req.CreateOrganization {
		// Create new organization
		if req.OrganizationName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "Organization name is required when creating an organization",
				},
			})
			return
		}

		slug := strings.ToLower(strings.ReplaceAll(req.OrganizationName, " ", "-"))
		org := &storage.Organization{
			Name:                req.OrganizationName,
			Slug:                slug,
			Tier:                "starter",
			MonthlyRequestLimit: 50_000,
		}

		if err := h.orgRepo.Create(c.Request.Context(), org); err != nil {
			if err == storage.ErrAlreadyExists {
				c.JSON(http.StatusConflict, gin.H{
					"error": gin.H{
						"code":    "SLUG_EXISTS",
						"message": "An organization with this name already exists",
					},
				})
				return
			}
			log.Printf("Failed to create organization: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "Failed to create organization",
				},
			})
			return
		}

		orgID = org.ID
	}

	result, err := h.authService.SignUp(c.Request.Context(), req.Email, req.Password, req.Name)
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

	// Create user in database
	user := &storage.User{
		Email:     req.Email,
		IDPSub:    result.UserSub,
		Name:      req.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		log.Printf("Failed to create user in database: %v", err)
		// Continue even if DB creation fails
	}

	// If organization was created or invite was used, add user as member
	if orgID != "" {
		role := storage.UserRoleAdmin
		if dbToken != "" {
			// Use role from invite
			invite, _ := h.inviteRepo.GetByToken(c.Request.Context(), dbToken)
			if invite != nil {
				role = invite.Role
			}
		}

		member := &storage.OrganizationMember{
			OrganizationID: orgID,
			UserID:         user.ID,
			Role:           role,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := h.memberRepo.Create(c.Request.Context(), member); err != nil {
			log.Printf("Failed to create organization member: %v", err)
			// Continue even if membership creation fails
		}

		// Accept the invite if one was used
		if dbToken != "" {
			if err := h.inviteRepo.Accept(c.Request.Context(), dbToken, user.ID); err != nil {
				log.Printf("Failed to accept invite: %v", err)
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":         true,
		"user_confirmed":  result.UserConfirmed,
		"organization_id": orgID,
		"message":         "Sign up successful. Please check your email for verification code.",
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

	if err := h.authService.ConfirmSignUp(c.Request.Context(), req.Email, req.Code); err != nil {
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

	tokens, err := h.authService.SignIn(c.Request.Context(), req.Email, req.Password)
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

	// Get user info from Cognito to sync with database
	userInfo, err := h.authService.GetUser(c.Request.Context(), tokens.AccessToken)
	if err != nil {
		log.Printf("Failed to get user info after signin: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to retrieve user information",
			},
		})
		return
	}

	// Create or update user in database
	if err := h.syncUserToDB(c.Request.Context(), userInfo); err != nil {
		log.Printf("Failed to sync user to database: %v", err)
		// Don't fail the signin, just log the error
	}

	// Handle pending invites
	if err := h.processPendingInvites(c.Request.Context(), userInfo); err != nil {
		log.Printf("Failed to process pending invites: %v", err)
		// Don't fail the signin, just log the error
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
		if err := h.authService.SignOut(c.Request.Context(), accessToken); err != nil {
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
	var userInfo *auth.UserInfo

	// Use ID token for authentication (works for both federated and native users)
	idToken, err := c.Cookie(idTokenCookie)
	if err != nil || idToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Not authenticated",
			},
		})
		return
	}

	userInfo, _, err = h.authService.ValidateIDToken(c.Request.Context(), idToken)
	if err != nil {
		log.Printf("ValidateIDToken error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "TOKEN_EXPIRED",
				"message": "Session expired, please refresh",
			},
		})
		return
	}

	// Get user from database to return our internal ID
	user, err := h.userRepo.GetByIDPSub(c.Request.Context(), userInfo.Sub)
	if err != nil {
		log.Printf("Failed to get user from database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to retrieve user information",
			},
		})
		return
	}

	// Generate profile picture URL
	// - If it's already a full URL (from Google/OAuth), use it directly
	// - If it's an S3 path (custom upload), generate CloudFront URL
	var profilePictureURL string
	if user.ProfilePicture != "" {
		if strings.HasPrefix(user.ProfilePicture, "http") {
			profilePictureURL = user.ProfilePicture
		} else if h.storageService != nil {
			profilePictureURL = h.storageService.GetCloudFrontURL(user.ProfilePicture)
		}
	} else if userInfo.Picture != "" {
		profilePictureURL = userInfo.Picture
	}

	// Get organization and role from database
	var organizationID string
	var role storage.UserRole
	memberships, err := h.memberRepo.ListByUser(c.Request.Context(), user.ID)
	if err == nil && len(memberships) > 0 {
		organizationID = memberships[0].OrganizationID
		role = memberships[0].Role
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":              user.ID,
			"sub":             userInfo.Sub,
			"email":           user.Email,
			"email_verified":  userInfo.EmailVerified,
			"name":            user.Name,
			"profile_picture": profilePictureURL,
			"role":            role,
			"organization_id": organizationID,
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

	tokens, err := h.authService.RefreshToken(c.Request.Context(), refreshToken)
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

// GoogleSignIn handles OAuth sign-in (Google via Cognito)
// The frontend sends the authorization code, backend exchanges it for tokens securely
// @Summary Sign in with Google (OAuth)
// @Description Exchanges OAuth authorization code for tokens and creates a session
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object{code=string,redirect_uri=string} true "OAuth authorization code and redirect URI"
// @Success 200 {object} object{success=bool,user=object}
// @Failure 400 {object} object{error=object{code=string,message=string}}
// @Failure 401 {object} object{error=object{code=string,message=string}}
// @Router /auth/google [post]
func (h *AuthHandler) GoogleSignIn(c *gin.Context) {
	var req struct {
		Code        string `json:"code" binding:"required"`
		RedirectURI string `json:"redirect_uri" binding:"required"`
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

	// Exchange the authorization code for tokens (server-side, with client secret)
	tokens, err := h.authService.ExchangeCodeForTokens(c.Request.Context(), req.Code, req.RedirectURI)
	if err != nil {
		log.Printf("ExchangeCodeForTokens error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "TOKEN_EXCHANGE_FAILED",
				"message": "Failed to exchange authorization code",
			},
		})
		return
	}

	// Validate the ID token and extract user info
	userInfo, _, err := h.authService.ValidateIDToken(c.Request.Context(), tokens.IDToken)
	if err != nil {
		log.Printf("ValidateIDToken error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "INVALID_TOKEN",
				"message": "Invalid or expired token",
			},
		})
		return
	}

	// Sync user to database
	if err := h.syncUserToDB(c.Request.Context(), userInfo); err != nil {
		log.Printf("Failed to sync OAuth user to database: %v", err)
	}

	// Get user from database
	user, err := h.userRepo.GetByIDPSub(c.Request.Context(), userInfo.Sub)
	if err != nil {
		log.Printf("Failed to get user from database after OAuth: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to retrieve user information",
			},
		})
		return
	}

	// Handle pending invites
	if err := h.processPendingInvites(c.Request.Context(), userInfo); err != nil {
		log.Printf("Failed to process pending invites: %v", err)
	}

	// Set HTTP-only secure session cookies
	h.setAuthCookies(c, tokens)

	// Generate profile picture URL
	// - If it's already a full URL (from Google/OAuth), use it directly
	// - If it's an S3 path (custom upload), generate CloudFront URL
	var profilePictureURL string
	if user.ProfilePicture != "" {
		if strings.HasPrefix(user.ProfilePicture, "http") {
			profilePictureURL = user.ProfilePicture
		} else if h.storageService != nil {
			profilePictureURL = h.storageService.GetCloudFrontURL(user.ProfilePicture)
		}
	} else if userInfo.Picture != "" {
		profilePictureURL = userInfo.Picture
	}

	// Get organization and role from database
	var organizationID string
	var role storage.UserRole
	memberships, err := h.memberRepo.ListByUser(c.Request.Context(), user.ID)
	if err == nil && len(memberships) > 0 {
		organizationID = memberships[0].OrganizationID
		role = memberships[0].Role
	}

	// Determine if user needs to set up an organization
	needsOrganization := organizationID == ""

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"needs_organization": needsOrganization,
		"user": gin.H{
			"id":              user.ID,
			"sub":             userInfo.Sub,
			"email":           user.Email,
			"email_verified":  userInfo.EmailVerified,
			"name":            user.Name,
			"profile_picture": profilePictureURL,
			"role":            role,
			"organization_id": organizationID,
		},
	})
}

// SetupOrganization creates an organization for authenticated users who don't have one
// @Summary Setup organization for current user
// @Description Creates an organization for the current user and adds them as admin
// @Tags auth
// @Accept json
// @Produce json
// @Param request body object{organization_name=string} true "Organization setup request"
// @Success 200 {object} object{success=bool,organization_id=string}
// @Failure 400 {object} object{error=object{code=string,message=string}}
// @Failure 401 {object} object{error=object{code=string,message=string}}
// @Router /auth/setup-organization [post]
func (h *AuthHandler) SetupOrganization(c *gin.Context) {
	var req struct {
		OrganizationName string `json:"organization_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Organization name is required",
			},
		})
		return
	}

	// Get user info from ID token
	idToken, err := c.Cookie(idTokenCookie)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Not authenticated",
			},
		})
		return
	}

	userInfo, _, err := h.authService.ValidateIDToken(c.Request.Context(), idToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Invalid or expired session",
			},
		})
		return
	}

	// Get user from database
	user, err := h.userRepo.GetByIDPSub(c.Request.Context(), userInfo.Sub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to retrieve user information",
			},
		})
		return
	}

	// Check if user already has an organization
	memberships, err := h.memberRepo.ListByUser(c.Request.Context(), user.ID)
	if err == nil && len(memberships) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "ALREADY_HAS_ORGANIZATION",
				"message": "User already belongs to an organization",
			},
		})
		return
	}

	// Create the organization
	slug := strings.ToLower(strings.ReplaceAll(req.OrganizationName, " ", "-"))
	org := &storage.Organization{
		Name:                req.OrganizationName,
		Slug:                slug,
		Tier:                "starter",
		MonthlyRequestLimit: 50_000,
	}

	if err := h.orgRepo.Create(c.Request.Context(), org); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "SLUG_EXISTS",
					"message": "An organization with this name already exists",
				},
			})
			return
		}
		log.Printf("Failed to create organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create organization",
			},
		})
		return
	}

	// Add user as admin member
	member := &storage.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Role:           storage.UserRoleAdmin,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.memberRepo.Create(c.Request.Context(), member); err != nil {
		log.Printf("Failed to create organization member: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to add user to organization",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"organization_id": org.ID,
		"organization": gin.H{
			"id":   org.ID,
			"name": org.Name,
			"slug": org.Slug,
			"tier": org.Tier,
		},
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
		h.cookieDomain,
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
		h.cookieDomain,
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
			h.cookieDomain,
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
			h.cookieDomain,
			h.secureCookies,
			true,
		)
	}
}

// syncUserToDB creates or updates a user in the database based on Cognito user info
func (h *AuthHandler) syncUserToDB(ctx context.Context, userInfo *auth.UserInfo) error {
	// Check if user exists by IDPSub
	existingUser, err := h.userRepo.GetByIDPSub(ctx, userInfo.Sub)
	if err != nil && err != storage.ErrNotFound {
		return fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil {
		// User exists, update if needed
		needsUpdate := false
		if existingUser.Email != userInfo.Email {
			existingUser.Email = userInfo.Email
			needsUpdate = true
		}
		// Update name if it was empty or different (prefer non-empty name from provider)
		if userInfo.Name != "" && (existingUser.Name == "" || existingUser.Name != userInfo.Name) {
			existingUser.Name = userInfo.Name
			needsUpdate = true
		}
		// Update profile picture from OAuth provider if user doesn't have a custom one
		// (custom profile pictures are S3 paths, OAuth pictures are URLs)
		if userInfo.Picture != "" && (existingUser.ProfilePicture == "" || strings.HasPrefix(existingUser.ProfilePicture, "http")) {
			existingUser.ProfilePicture = userInfo.Picture
			needsUpdate = true
		}
		if needsUpdate {
			existingUser.UpdatedAt = time.Now()
			if err := h.userRepo.Update(ctx, existingUser); err != nil {
				return fmt.Errorf("failed to update user: %w", err)
			}
		}
		return nil
	}

	// User not found by IDPSub - check if they exist by email (migration case)
	existingUser, err = h.userRepo.GetByEmail(ctx, userInfo.Email)
	if err != nil && err != storage.ErrNotFound {
		return fmt.Errorf("failed to check existing user by email: %w", err)
	}

	if existingUser != nil {
		// User exists with this email but different/missing IDPSub - update it
		existingUser.IDPSub = userInfo.Sub
		if userInfo.Name != "" {
			existingUser.Name = userInfo.Name
		}
		// Update profile picture if user doesn't have a custom one
		if userInfo.Picture != "" && (existingUser.ProfilePicture == "" || strings.HasPrefix(existingUser.ProfilePicture, "http")) {
			existingUser.ProfilePicture = userInfo.Picture
		}
		existingUser.UpdatedAt = time.Now()
		if err := h.userRepo.Update(ctx, existingUser); err != nil {
			return fmt.Errorf("failed to update user IDPSub: %w", err)
		}
		return nil
	}

	// Create new user
	newUser := &storage.User{
		Email:          userInfo.Email,
		IDPSub:         userInfo.Sub,
		Name:           userInfo.Name,
		ProfilePicture: userInfo.Picture,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.userRepo.Create(ctx, newUser); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// processPendingInvites checks for pending invites and accepts them
func (h *AuthHandler) processPendingInvites(ctx context.Context, userInfo *auth.UserInfo) error {
	// Get user from database
	_, err := h.userRepo.GetByIDPSub(ctx, userInfo.Sub)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Check if there's a pending invite for this email
	// Note: We'd need to add a method to list invites by email across all orgs
	// For now, we'll skip this and handle invite acceptance separately through the invite token flow

	return nil
}
