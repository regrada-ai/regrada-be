package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type UserHandler struct {
	userRepo   storage.UserRepository
	memberRepo storage.OrganizationMemberRepository
}

func NewUserHandler(userRepo storage.UserRepository, memberRepo storage.OrganizationMemberRepository) *UserHandler {
	return &UserHandler{
		userRepo:   userRepo,
		memberRepo: memberRepo,
	}
}

// GetCurrentUser retrieves the current authenticated user
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	idpSub := c.GetString("sub")
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
					"code":    "NOT_FOUND",
					"message": "User not found",
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

	memberships, err := h.memberRepo.ListByUser(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch user memberships",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":        user,
		"memberships": memberships,
	})
}

// GetUser retrieves a user by ID
func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("userID")

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "User not found",
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

	c.JSON(http.StatusOK, user)
}

// ListOrganizationUsers lists all users in an organization
func (h *UserHandler) ListOrganizationUsers(c *gin.Context) {
	orgID := c.Param("orgID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot list users from a different organization",
			},
		})
		return
	}

	members, err := h.memberRepo.ListByOrganization(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch organization members",
			},
		})
		return
	}

	type UserWithRole struct {
		*storage.User
		Role storage.UserRole `json:"role"`
	}

	usersWithRoles := make([]UserWithRole, 0, len(members))
	for _, member := range members {
		user, err := h.userRepo.GetByID(c.Request.Context(), member.UserID)
		if err != nil {
			continue
		}
		usersWithRoles = append(usersWithRoles, UserWithRole{
			User: user,
			Role: member.Role,
		})
	}

	c.JSON(http.StatusOK, usersWithRoles)
}

// UpdateUser updates a user
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("userID")

	idpSub := c.GetString("sub")
	currentUser, err := h.userRepo.GetByIDPSub(c.Request.Context(), idpSub)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "User authentication required",
			},
		})
		return
	}

	if currentUser.ID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot update a different user",
			},
		})
		return
	}

	var req struct {
		Name  *string `json:"name"`
		Email *string `json:"email"`
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

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "User not found",
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

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		user.Email = *req.Email
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to update user",
			},
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// DeleteUser soft deletes a user
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("userID")

	idpSub := c.GetString("sub")
	currentUser, err := h.userRepo.GetByIDPSub(c.Request.Context(), idpSub)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "User authentication required",
			},
		})
		return
	}

	if currentUser.ID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot delete a different user",
			},
		})
		return
	}

	if err := h.userRepo.Delete(c.Request.Context(), userID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "User not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to delete user",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// UpdateOrganizationMemberRole updates a user's role in an organization
func (h *UserHandler) UpdateOrganizationMemberRole(c *gin.Context) {
	orgID := c.Param("orgID")
	userID := c.Param("userID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot update members in a different organization",
			},
		})
		return
	}

	var req struct {
		Role storage.UserRole `json:"role" binding:"required,oneof=admin user readonly-user"`
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

	member, err := h.memberRepo.GetByUserAndOrg(c.Request.Context(), userID, orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Member not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch member",
			},
		})
		return
	}

	if err := h.memberRepo.UpdateRole(c.Request.Context(), member.ID, req.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to update member role",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// RemoveOrganizationMember removes a user from an organization
func (h *UserHandler) RemoveOrganizationMember(c *gin.Context) {
	orgID := c.Param("orgID")
	userID := c.Param("userID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot remove members from a different organization",
			},
		})
		return
	}

	member, err := h.memberRepo.GetByUserAndOrg(c.Request.Context(), userID, orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Member not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch member",
			},
		})
		return
	}

	if err := h.memberRepo.Delete(c.Request.Context(), member.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to remove member",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
