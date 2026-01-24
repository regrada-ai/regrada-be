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
// @Summary      Upload a test run
// @Description  Upload a test run with test results for a project
// @Tags         test-runs
// @Accept       json
// @Produce      json
// @Param        projectID  path      string           true  "Project ID"
// @Param        testRun    body      regrada.TestRun  true  "Test run data"
// @Success      201        {object}  map[string]interface{} "Test run created successfully"
// @Failure      400        {object}  map[string]interface{} "Invalid request"
// @Failure      401        {object}  map[string]interface{} "Unauthorized"
// @Failure      500        {object}  map[string]interface{} "Internal server error"
// @Security     BearerAuth
// @Router       /v1/projects/{projectID}/test-runs [post]
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
// @Summary      List test runs
// @Description  Get a paginated list of test runs for a project
// @Tags         test-runs
// @Accept       json
// @Produce      json
// @Param        projectID  path      string  true  "Project ID"
// @Success      200        {object}  map[string]interface{} "List of test runs"
// @Failure      401        {object}  map[string]interface{} "Unauthorized"
// @Failure      500        {object}  map[string]interface{} "Internal server error"
// @Security     BearerAuth
// @Router       /v1/projects/{projectID}/test-runs [get]
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
// @Summary      Get a test run
// @Description  Get a specific test run by ID
// @Tags         test-runs
// @Accept       json
// @Produce      json
// @Param        projectID  path      string  true  "Project ID"
// @Param        runID      path      string  true  "Test Run ID"
// @Success      200        {object}  regrada.TestRun "Test run details"
// @Failure      401        {object}  map[string]interface{} "Unauthorized"
// @Failure      404        {object}  map[string]interface{} "Test run not found"
// @Failure      500        {object}  map[string]interface{} "Internal server error"
// @Security     BearerAuth
// @Router       /v1/projects/{projectID}/test-runs/{runID} [get]
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
