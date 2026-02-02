package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/regrada-ai/regrada-be/internal/storage/s3"
)

type UserHandler struct {
	userRepo       storage.UserRepository
	memberRepo     storage.OrganizationMemberRepository
	storageService storage.FileStorageService
}

func NewUserHandler(userRepo storage.UserRepository, memberRepo storage.OrganizationMemberRepository, storageService storage.FileStorageService) *UserHandler {
	return &UserHandler{
		userRepo:       userRepo,
		memberRepo:     memberRepo,
		storageService: storageService,
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

	// Generate CloudFront URL for profile picture if it exists
	var profilePictureURL string
	if user.ProfilePicture != "" && h.storageService != nil {
		profilePictureURL = h.storageService.GetCloudFrontURL(user.ProfilePicture)
	}

	// Create response with presigned URL
	userResponse := gin.H{
		"id":              user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"profile_picture": profilePictureURL,
		"role":            user.Role,
		"created_at":      user.CreatedAt,
		"updated_at":      user.UpdatedAt,
	}

	c.JSON(http.StatusOK, gin.H{
		"user":        userResponse,
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

	// Generate CloudFront URL for profile picture if it exists
	var profilePictureURL string
	if user.ProfilePicture != "" && h.storageService != nil {
		profilePictureURL = h.storageService.GetCloudFrontURL(user.ProfilePicture)
	}

	userResponse := gin.H{
		"id":              user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"profile_picture": profilePictureURL,
		"role":            user.Role,
		"created_at":      user.CreatedAt,
		"updated_at":      user.UpdatedAt,
	}

	c.JSON(http.StatusOK, userResponse)
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

// UploadProfilePicture uploads a profile picture to S3
func (h *UserHandler) UploadProfilePicture(c *gin.Context) {
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
				"message": "Cannot upload profile picture for a different user",
			},
		})
		return
	}

	if h.storageService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "S3_NOT_CONFIGURED",
				"message": "File upload service is not configured",
			},
		})
		return
	}

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_FILE",
				"message": "No file provided",
			},
		})
		return
	}
	defer file.Close()

	// Validate image file
	if err := s3.ValidateImageFile(header); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_FILE",
				"message": err.Error(),
			},
		})
		return
	}

	// Generate S3 key
	ext := s3.GetFileExtension(header.Filename)
	s3Key := fmt.Sprintf("users/%s/profile%s", userID, ext)

	// Delete old profile picture if exists
	if currentUser.ProfilePicture != "" {
		// Extract S3 key from URL or use as-is if it's already a key
		oldKey := currentUser.ProfilePicture
		if err := h.storageService.DeleteFile(c.Request.Context(), oldKey); err != nil {
			// Log error but don't fail the request
			// The old file might not exist or might be a URL from another source
		}
	}

	// Upload to S3
	contentType := header.Header.Get("Content-Type")
	if err := h.storageService.UploadFile(c.Request.Context(), s3Key, file, contentType); err != nil {
		fmt.Printf("S3 upload failed for key %s: %v\n", s3Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "UPLOAD_FAILED",
				"message": fmt.Sprintf("Failed to upload file: %v", err),
			},
		})
		return
	}

	// Update user's profile picture in database
	currentUser.ProfilePicture = s3Key
	if err := h.userRepo.Update(c.Request.Context(), currentUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to update user profile",
			},
		})
		return
	}

	// Generate CloudFront URL
	cloudFrontURL := h.storageService.GetCloudFrontURL(s3Key)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"s3_key":          s3Key,
		"url":             cloudFrontURL,
		"profile_picture": cloudFrontURL,
	})
}

// DeleteProfilePicture deletes a user's profile picture from S3
func (h *UserHandler) DeleteProfilePicture(c *gin.Context) {
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
				"message": "Cannot delete profile picture for a different user",
			},
		})
		return
	}

	if currentUser.ProfilePicture == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "No profile picture to delete",
			},
		})
		return
	}

	// Delete from S3 if service is configured
	if h.storageService != nil {
		if err := h.storageService.DeleteFile(c.Request.Context(), currentUser.ProfilePicture); err != nil {
			// Log error but continue to remove from database
			// The file might have already been deleted or might not exist
		}
	}

	// Remove profile picture reference from database
	currentUser.ProfilePicture = ""
	if err := h.userRepo.Update(c.Request.Context(), currentUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to update user profile",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile picture deleted successfully",
	})
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

	// Generate CloudFront URL for profile picture if it exists
	var profilePictureURL string
	if user.ProfilePicture != "" && h.storageService != nil {
		profilePictureURL = h.storageService.GetCloudFrontURL(user.ProfilePicture)
	}

	userResponse := gin.H{
		"id":              user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"profile_picture": profilePictureURL,
		"role":            user.Role,
		"created_at":      user.CreatedAt,
		"updated_at":      user.UpdatedAt,
	}

	c.JSON(http.StatusOK, userResponse)
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
