package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type InviteHandler struct {
	inviteRepo storage.InviteRepository
	userRepo   storage.UserRepository
	memberRepo storage.OrganizationMemberRepository
}

func NewInviteHandler(
	inviteRepo storage.InviteRepository,
	userRepo storage.UserRepository,
	memberRepo storage.OrganizationMemberRepository,
) *InviteHandler {
	return &InviteHandler{
		inviteRepo: inviteRepo,
		userRepo:   userRepo,
		memberRepo: memberRepo,
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
		Role      storage.UserRole `json:"role" binding:"required,oneof=admin user readonly-user"`
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
	idpSub := c.GetString("user_id")
	log.Printf("DEBUG CreateInvite: user_id=\x27%s\x27", idpSub)

	invitingUser, err := h.userRepo.GetByIDPSub(c.Request.Context(), idpSub)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "User authentication required",
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

	c.JSON(http.StatusCreated, gin.H{
		"id":         invite.ID,
		"email":      invite.Email,
		"role":       invite.Role,
		"token":      invite.Token,
		"expires_at": invite.ExpiresAt,
		"created_at": invite.CreatedAt,
	})
}

// ListInvites lists all invites for an organization
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

	c.JSON(http.StatusOK, invites)
}

// GetInvite retrieves invite details by token (public endpoint for invite acceptance)
func (h *InviteHandler) GetInvite(c *gin.Context) {
	token := c.Param("token")

	invite, err := h.inviteRepo.GetByToken(c.Request.Context(), token)
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

	// Return limited information for security
	c.JSON(http.StatusOK, gin.H{
		"organization_id": invite.OrganizationID,
		"email":           invite.Email,
		"role":            invite.Role,
		"expires_at":      invite.ExpiresAt,
		"is_accepted":     invite.AcceptedAt != nil,
		"is_revoked":      invite.RevokedAt != nil,
		"is_expired":      time.Now().After(invite.ExpiresAt),
	})
}

// AcceptInvite accepts an invitation and adds the user to the organization
func (h *InviteHandler) AcceptInvite(c *gin.Context) {
	token := c.Param("token")

	idpSub := c.GetString("user_id")
	if idpSub == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "User authentication required",
			},
		})
		return
	}

	user, err := h.userRepo.GetByIDPSub(c.Request.Context(), idpSub)
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

	invite, err := h.inviteRepo.GetByToken(c.Request.Context(), token)
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

	// Check if user is already a member
	existingMember, err := h.memberRepo.GetByUserAndOrg(c.Request.Context(), user.ID, invite.OrganizationID)
	if err != nil && err != storage.ErrNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to check existing membership",
			},
		})
		return
	}

	if existingMember != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": gin.H{
				"code":    "ALREADY_MEMBER",
				"message": "User is already a member of this organization",
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
	if err := h.inviteRepo.Accept(c.Request.Context(), token, user.ID); err != nil {
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
