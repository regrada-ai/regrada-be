// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type OrganizationHandler struct {
	orgRepo storage.OrganizationRepository
}

func NewOrganizationHandler(orgRepo storage.OrganizationRepository) *OrganizationHandler {
	return &OrganizationHandler{
		orgRepo: orgRepo,
	}
}

// CreateOrganization creates a new organization
func (h *OrganizationHandler) CreateOrganization(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Slug string `json:"slug" binding:"required"`
		Tier string `json:"tier" binding:"required,oneof=standard pro enterprise"`
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
		Tier: req.Tier,
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
