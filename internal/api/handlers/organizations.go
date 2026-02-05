// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"

	_ "github.com/regrada-ai/regrada-be/internal/api/types" // for swagger
)

type OrganizationHandler struct {
	orgRepo    storage.OrganizationRepository
	memberRepo storage.OrganizationMemberRepository
	userRepo   storage.UserRepository
	apiKeyRepo storage.APIKeyRepository
}

func NewOrganizationHandler(orgRepo storage.OrganizationRepository, memberRepo storage.OrganizationMemberRepository, userRepo storage.UserRepository, apiKeyRepo storage.APIKeyRepository) *OrganizationHandler {
	return &OrganizationHandler{
		orgRepo:    orgRepo,
		memberRepo: memberRepo,
		userRepo:   userRepo,
		apiKeyRepo: apiKeyRepo,
	}
}

// getUserOrganizationID looks up the user's organization from the database
func (h *OrganizationHandler) getUserOrganizationID(c *gin.Context) (string, error) {
	// First try to get user from database by their IDP subject
	sub := c.GetString("sub")
	if sub == "" {
		return "", errors.New("user sub not found in context")
	}

	user, err := h.userRepo.GetByIDPSub(c.Request.Context(), sub)
	if err != nil {
		return "", err
	}

	// Get organization membership from database
	memberships, err := h.memberRepo.ListByUser(c.Request.Context(), user.ID)
	if err != nil {
		return "", err
	}

	if len(memberships) == 0 {
		return "", nil
	}

	return memberships[0].OrganizationID, nil
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
		Name:                req.Name,
		Slug:                req.Slug,
		Tier:                "starter",
		MonthlyRequestLimit: monthlyLimitForTier("starter"),
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

	c.JSON(http.StatusCreated, org)
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

	log.Printf("ListOrganizations: fetching orgs for user_id=%s", userID)
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

	// Ensure we return an empty array, not null
	if orgs == nil {
		orgs = []*storage.Organization{}
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

	// Look up user's organization from database
	userOrgID, err := h.getUserOrganizationID(c)
	if err != nil {
		log.Printf("Failed to get user organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to verify organization membership",
			},
		})
		return
	}

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

	oldTier := org.Tier

	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.Slug != nil {
		org.Slug = *req.Slug
	}
	if req.Tier != nil {
		// Validate tier value
		newTier := *req.Tier
		if newTier != "starter" && newTier != "team" && newTier != "scale" && newTier != "enterprise" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "tier must be starter, team, scale, or enterprise",
				},
			})
			return
		}
		org.Tier = newTier
		org.MonthlyRequestLimit = monthlyLimitForTier(newTier)
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

	// If tier changed, update all API keys for this organization
	if oldTier != org.Tier {
		rateLimitRPM := rateLimitForTier(org.Tier)
		if err := h.apiKeyRepo.UpdateTierByOrganization(c.Request.Context(), orgID, org.Tier, rateLimitRPM); err != nil {
			log.Printf("Failed to update API key tiers for organization %s: %v", orgID, err)
			// Don't fail the request, the org update succeeded
		}
	}

	c.JSON(http.StatusOK, org)
}

// DeleteOrganization soft deletes an organization
func (h *OrganizationHandler) DeleteOrganization(c *gin.Context) {
	orgID := c.Param("orgID")

	// Look up user's organization from database
	userOrgID, err := h.getUserOrganizationID(c)
	if err != nil {
		log.Printf("Failed to get user organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to verify organization membership",
			},
		})
		return
	}

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
