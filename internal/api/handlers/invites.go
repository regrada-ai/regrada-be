package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/email"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type InviteHandler struct {
	inviteRepo   storage.InviteRepository
	userRepo     storage.UserRepository
	memberRepo   storage.OrganizationMemberRepository
	orgRepo      storage.OrganizationRepository
	emailService *email.Service
}

func NewInviteHandler(
	inviteRepo storage.InviteRepository,
	userRepo storage.UserRepository,
	memberRepo storage.OrganizationMemberRepository,
	orgRepo storage.OrganizationRepository,
	emailService *email.Service,
) *InviteHandler {
	return &InviteHandler{
		inviteRepo:   inviteRepo,
		userRepo:     userRepo,
		memberRepo:   memberRepo,
		orgRepo:      orgRepo,
		emailService: emailService,
	}
}

// CreateInvite creates a new organization invite
func (h *InviteHandler) CreateInvite(c *gin.Context) {
	orgID := c.Param("orgID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot create invites for a different organization",
			},
		})
		return
	}

	var req struct {
		Email     string           `json:"email" binding:"required,email"`
		Role      storage.UserRole `json:"role" binding:"required,oneof=admin member viewer"`
		ExpiresIn int              `json:"expires_in_hours"` // Hours until expiration, default 168 (7 days)
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

	// Check if there's already a pending invite for this email
	existingInvite, err := h.inviteRepo.GetByEmailAndOrg(c.Request.Context(), req.Email, orgID)
	if err != nil && err != storage.ErrNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to check existing invites",
			},
		})
		return
	}

	if existingInvite != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": gin.H{
				"code":    "INVITE_EXISTS",
				"message": "A pending invite already exists for this email",
			},
		})
		return
	}

	// Get inviting user
	userID := c.GetString("user_id")

	invitingUser, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "User authentication required",
			},
		})
		return
	}

	// Get organization for the email
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

	// Generate secure token
	token, err := generateInviteToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to generate invite token",
			},
		})
		return
	}

	// Default expiration to 7 days
	expiresIn := req.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 168 // 7 days
	}

	invite := &storage.Invite{
		OrganizationID: orgID,
		Email:          req.Email,
		Role:           req.Role,
		Token:          token,
		InvitedBy:      &invitingUser.ID,
		ExpiresAt:      time.Now().Add(time.Duration(expiresIn) * time.Hour),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.inviteRepo.Create(c.Request.Context(), invite); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create invite",
			},
		})
		return
	}

	// Create encoded token with email for the response and email
	encodedToken := encodeInviteToken(req.Email, token)

	// Send invite email (non-blocking, don't fail the request if email fails)
	if h.emailService != nil {
		go func(toEmail, inviterName, orgName, role, inviteToken string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := h.emailService.SendInviteEmail(
				ctx,
				toEmail,
				inviterName,
				orgName,
				role,
				inviteToken,
			)
			if err != nil {
				log.Printf("Failed to send invite email to %s: %v", toEmail, err)
			}
		}(req.Email, invitingUser.Name, org.Name, string(req.Role), encodedToken)
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         invite.ID,
		"email":      invite.Email,
		"role":       invite.Role,
		"token":      encodedToken,
		"expires_at": invite.ExpiresAt,
		"created_at": invite.CreatedAt,
	})
}

// ListInvites lists invites for an organization
// Query params:
//   - status: "pending" (default), "accepted", "revoked", "expired", or "all"
func (h *InviteHandler) ListInvites(c *gin.Context) {
	orgID := c.Param("orgID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot list invites for a different organization",
			},
		})
		return
	}

	invites, err := h.inviteRepo.ListByOrganization(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch invites",
			},
		})
		return
	}

	// Filter by status
	status := c.DefaultQuery("status", "pending")
	now := time.Now()

	type InviteResponse struct {
		ID             string     `json:"id"`
		Email          string     `json:"email"`
		Role           string     `json:"role"`
		Token          string     `json:"token"`
		InvitedBy      *string    `json:"invited_by,omitempty"`
		AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
		RevokedAt      *time.Time `json:"revoked_at,omitempty"`
		ExpiresAt      time.Time  `json:"expires_at"`
		CreatedAt      time.Time  `json:"created_at"`
		Status         string     `json:"status"`
	}

	var filtered []InviteResponse
	for _, inv := range invites {
		// Determine status
		var invStatus string
		if inv.AcceptedAt != nil {
			invStatus = "accepted"
		} else if inv.RevokedAt != nil {
			invStatus = "revoked"
		} else if now.After(inv.ExpiresAt) {
			invStatus = "expired"
		} else {
			invStatus = "pending"
		}

		// Filter based on query param
		if status != "all" && invStatus != status {
			continue
		}

		filtered = append(filtered, InviteResponse{
			ID:         inv.ID,
			Email:      inv.Email,
			Role:       string(inv.Role),
			Token:      encodeInviteToken(inv.Email, inv.Token),
			InvitedBy:  inv.InvitedBy,
			AcceptedAt: inv.AcceptedAt,
			RevokedAt:  inv.RevokedAt,
			ExpiresAt:  inv.ExpiresAt,
			CreatedAt:  inv.CreatedAt,
			Status:     invStatus,
		})
	}

	if filtered == nil {
		filtered = []InviteResponse{}
	}

	c.JSON(http.StatusOK, filtered)
}

// GetInvite retrieves invite details by token (public endpoint for invite acceptance)
func (h *InviteHandler) GetInvite(c *gin.Context) {
	token := c.Param("token")

	// Decode the token to extract the random part for DB lookup
	_, dbToken, err := decodeInviteToken(token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_TOKEN",
				"message": "Invalid invite token format",
			},
		})
		return
	}

	invite, err := h.inviteRepo.GetByToken(c.Request.Context(), dbToken)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Invite not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch invite",
			},
		})
		return
	}

	// Fetch organization details
	var orgName string
	org, err := h.orgRepo.Get(c.Request.Context(), invite.OrganizationID)
	if err == nil {
		orgName = org.Name
	}

	// Fetch inviter details
	var inviterName string
	if invite.InvitedBy != nil {
		inviter, err := h.userRepo.GetByID(c.Request.Context(), *invite.InvitedBy)
		if err == nil {
			inviterName = inviter.Name
		}
	}

	// Return invite information
	c.JSON(http.StatusOK, gin.H{
		"organization_id":   invite.OrganizationID,
		"organization_name": orgName,
		"email":             invite.Email,
		"role":              invite.Role,
		"invited_by":        inviterName,
		"expires_at":        invite.ExpiresAt,
		"is_accepted":       invite.AcceptedAt != nil,
		"is_revoked":        invite.RevokedAt != nil,
		"is_expired":        time.Now().After(invite.ExpiresAt),
	})
}

// AcceptInvite accepts an invitation and adds the user to the organization
func (h *InviteHandler) AcceptInvite(c *gin.Context) {
	token := c.Param("token")

	// Decode the token to extract the random part for DB lookup
	_, dbToken, err := decodeInviteToken(token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_TOKEN",
				"message": "Invalid invite token format",
			},
		})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "User authentication required",
			},
		})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "USER_NOT_FOUND",
					"message": "User not found in database",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch user",
			},
		})
		return
	}

	invite, err := h.inviteRepo.GetByToken(c.Request.Context(), dbToken)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "INVITE_NOT_FOUND",
					"message": "Invite not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch invite",
			},
		})
		return
	}

	// Validate invite
	if invite.AcceptedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVITE_ALREADY_ACCEPTED",
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

	// Check if user already belongs to an organization (users can only be in one org)
	existingMemberships, err := h.memberRepo.ListByUser(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to check existing membership",
			},
		})
		return
	}

	if len(existingMemberships) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": gin.H{
				"code":    "ALREADY_HAS_ORGANIZATION",
				"message": "User already belongs to an organization",
			},
		})
		return
	}

	// Create organization membership
	member := &storage.OrganizationMember{
		OrganizationID: invite.OrganizationID,
		UserID:         user.ID,
		Role:           invite.Role,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.memberRepo.Create(c.Request.Context(), member); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create membership",
			},
		})
		return
	}

	// Mark invite as accepted
	if err := h.inviteRepo.Accept(c.Request.Context(), dbToken, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to accept invite",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"organization_id": invite.OrganizationID,
		"role":            invite.Role,
	})
}

// RevokeInvite revokes an invitation
func (h *InviteHandler) RevokeInvite(c *gin.Context) {
	orgID := c.Param("orgID")
	inviteID := c.Param("inviteID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot revoke invites for a different organization",
			},
		})
		return
	}

	if err := h.inviteRepo.Revoke(c.Request.Context(), inviteID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Invite not found or already accepted/revoked",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to revoke invite",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// generateInviteToken generates a secure random token for invites
func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// encodeInviteToken creates a compound token that includes the email
// Format: base64url(email|randomToken)
func encodeInviteToken(email, token string) string {
	compound := email + "|" + token
	return base64.URLEncoding.EncodeToString([]byte(compound))
}

// decodeInviteToken extracts the email and random token from a compound token
// Returns the email, the random token (for DB lookup), and any error
func decodeInviteToken(compoundToken string) (email string, token string, err error) {
	// First try to decode as a compound token
	decoded, err := base64.URLEncoding.DecodeString(compoundToken)
	if err != nil {
		// Not a valid base64, return as-is (legacy token)
		return "", compoundToken, nil
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		// Not a compound token, might be a legacy token
		// Check if it looks like a raw token (base64 encoded 32 bytes = 44 chars)
		if len(compoundToken) == 44 {
			return "", compoundToken, nil
		}
		return "", "", fmt.Errorf("invalid token format")
	}

	return parts[0], parts[1], nil
}
