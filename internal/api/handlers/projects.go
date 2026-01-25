package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type ProjectHandler struct {
	projectRepo storage.ProjectRepository
}

func NewProjectHandler(projectRepo storage.ProjectRepository) *ProjectHandler {
	return &ProjectHandler{
		projectRepo: projectRepo,
	}
}

// CreateProject creates a new project
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req struct {
		OrganizationID string `json:"organization_id,omitempty"`
		Name           string `json:"name" binding:"required"`
		Slug           string `json:"slug" binding:"required"`
		GitHubOwner    string `json:"github_owner"`
		GitHubRepo     string `json:"github_repo"`
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

	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	if req.OrganizationID != "" && req.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Organization mismatch",
			},
		})
		return
	}

	project := &storage.Project{
		OrganizationID: orgID,
		Name:           req.Name,
		Slug:           req.Slug,
	}

	if err := h.projectRepo.Create(c.Request.Context(), project); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "ALREADY_EXISTS",
					"message": "Project with this slug already exists",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create project",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, project)
}

// GetProject retrieves a project by ID
func (h *ProjectHandler) GetProject(c *gin.Context) {
	projectID := c.Param("projectID")

	project, err := h.projectRepo.Get(c.Request.Context(), projectID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Project not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch project",
			},
		})
		return
	}

	c.JSON(http.StatusOK, project)
}

// ListProjects lists all projects for an organization
func (h *ProjectHandler) ListProjects(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Organization not found in token",
			},
		})
		return
	}

	projects, err := h.projectRepo.ListByOrganization(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch projects",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"count":    len(projects),
	})
}
