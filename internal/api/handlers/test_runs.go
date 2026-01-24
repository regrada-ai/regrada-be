package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/regrada-ai/regrada-be/pkg/regrada"
)

type TestRunHandler struct {
	testRunRepo storage.TestRunRepository
	projectRepo storage.ProjectRepository
}

func NewTestRunHandler(testRunRepo storage.TestRunRepository, projectRepo storage.ProjectRepository) *TestRunHandler {
	return &TestRunHandler{
		testRunRepo: testRunRepo,
		projectRepo: projectRepo,
	}
}

// UploadTestRun handles test run upload
func (h *TestRunHandler) UploadTestRun(c *gin.Context) {
	projectID := c.Param("projectID")

	var testRun regrada.TestRun
	if err := c.ShouldBindJSON(&testRun); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid test run data",
			},
		})
		return
	}

	// Store test run
	if err := h.testRunRepo.Create(c.Request.Context(), projectID, &testRun); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to store test run",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "created",
		"run_id": testRun.RunID,
	})
}

// ListTestRuns returns paginated list of test runs
func (h *TestRunHandler) ListTestRuns(c *gin.Context) {
	projectID := c.Param("projectID")

	// TODO: Add pagination and filtering
	testRuns, err := h.testRunRepo.List(c.Request.Context(), projectID, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch test runs",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"test_runs": testRuns,
		"count":     len(testRuns),
	})
}

// GetTestRun returns a single test run
func (h *TestRunHandler) GetTestRun(c *gin.Context) {
	projectID := c.Param("projectID")
	runID := c.Param("runID")

	testRun, err := h.testRunRepo.Get(c.Request.Context(), projectID, runID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Test run not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch test run",
			},
		})
		return
	}

	c.JSON(http.StatusOK, testRun)
}
