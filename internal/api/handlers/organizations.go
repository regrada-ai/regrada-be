package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/auth"
	"github.com/regrada-ai/regrada-be/internal/storage"

	_ "github.com/regrada-ai/regrada-be/internal/api/types" // for swagger
)

type OrganizationHandler struct {
	orgRepo     storage.OrganizationRepository
	authService auth.Service
}

func NewOrganizationHandler(orgRepo storage.OrganizationRepository, authService auth.Service) *OrganizationHandler {
	return &OrganizationHandler{
		orgRepo:     orgRepo,
		authService: authService,
	}
}

// CreateOrganization creates a new organization
// @Summary Create a new organization
// @Description Create a new organization
// @Tags organizations
// @Accept json
// @Produce json
// @Param request body types.CreateOrganizationRequest true "Organization details"
// @Success 201 {object} types.Organization
// @Failure 400 {object} types.ErrorResponse
// @Failure 409 {object} types.ErrorResponse
// @Router /organizations [post]
func (h *OrganizationHandler) CreateOrganization(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Slug string `json:"slug" binding:"required"`
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

	org := &storage.Organization{
		Name: req.Name,
		Slug: req.Slug,
		Tier: "free", // Always create orgs in the lowest tier
	}

	if err := h.orgRepo.Create(c.Request.Context(), org); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "ALREADY_EXISTS",
					"message": "Organization with this slug already exists",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create organization",
			},
		})
		return
	}

	if h.authService != nil {
		accessToken, err := c.Cookie(accessTokenCookie)
		if err == nil && accessToken != "" {
			if err := h.authService.UpdateUserOrganization(c.Request.Context(), accessToken, org.ID); err != nil {
				log.Printf("Failed to link organization to Cognito user: %v", err)
			}
		}
	}

	c.JSON(http.StatusCreated, org)
}

// InviteUser assigns a user to an organization via Cognito.
func (h *OrganizationHandler) InviteUser(c *gin.Context) {
	if h.authService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Auth service is not configured",
			},
		})
		return
	}

	orgID := c.Param("orgID")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "organization ID is required",
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

	userOrgID := c.GetString("organization_id")
	if userOrgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot invite users to a different organization",
			},
		})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
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

	if err := h.authService.AdminUpdateUserOrganization(c.Request.Context(), req.Email, orgID); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "User not found",
				},
			})
			return
		}

		log.Printf("Failed to invite user to organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to invite user",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// ListOrganizations retrieves all organizations for the authenticated user
// @Summary List organizations for user
// @Description Get all organizations that the authenticated user belongs to
// @Tags organizations
// @Produce json
// @Success 200 {array} types.Organization
// @Failure 401 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /organizations [get]
func (h *OrganizationHandler) ListOrganizations(c *gin.Context) {
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

	orgs, err := h.orgRepo.GetByUser(c.Request.Context(), userID)
	if err != nil {
		log.Printf("Failed to fetch organizations for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch organizations",
			},
		})
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// GetOrganization retrieves an organization by ID
func (h *OrganizationHandler) GetOrganization(c *gin.Context) {
	orgID := c.Param("orgID")

	org, err := h.orgRepo.Get(c.Request.Context(), orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Organization not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch organization",
			},
		})
		return
	}

	c.JSON(http.StatusOK, org)
}

// UpdateOrganization updates an organization
func (h *OrganizationHandler) UpdateOrganization(c *gin.Context) {
	orgID := c.Param("orgID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot update a different organization",
			},
		})
		return
	}

	var req struct {
		Name          *string `json:"name"`
		Slug          *string `json:"slug"`
		Tier          *string `json:"tier"`
		GitHubOrgID   *int64  `json:"github_org_id"`
		GitHubOrgName *string `json:"github_org_name"`
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

	org, err := h.orgRepo.Get(c.Request.Context(), orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Organization not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch organization",
			},
		})
		return
	}

	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.Slug != nil {
		org.Slug = *req.Slug
	}
	if req.Tier != nil {
		org.Tier = *req.Tier
	}
	if req.GitHubOrgID != nil {
		org.GitHubOrgID = req.GitHubOrgID
	}
	if req.GitHubOrgName != nil {
		org.GitHubOrgName = *req.GitHubOrgName
	}

	if err := h.orgRepo.Update(c.Request.Context(), org); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to update organization",
			},
		})
		return
	}

	c.JSON(http.StatusOK, org)
}

// DeleteOrganization soft deletes an organization
func (h *OrganizationHandler) DeleteOrganization(c *gin.Context) {
	orgID := c.Param("orgID")

	userOrgID := c.GetString("organization_id")
	if userOrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Cannot delete a different organization",
			},
		})
		return
	}

	if err := h.orgRepo.Delete(c.Request.Context(), orgID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Organization not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to delete organization",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
