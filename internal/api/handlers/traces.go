package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/regrada-ai/regrada-be/pkg/regrada"
)

type TraceHandler struct {
	traceRepo   storage.TraceRepository
	projectRepo storage.ProjectRepository
}

func NewTraceHandler(traceRepo storage.TraceRepository, projectRepo storage.ProjectRepository) *TraceHandler {
	return &TraceHandler{
		traceRepo:   traceRepo,
		projectRepo: projectRepo,
	}
}

// UploadTrace handles single trace upload
func (h *TraceHandler) UploadTrace(c *gin.Context) {
	projectID := c.Param("projectID")

	var trace regrada.Trace
	if err := c.ShouldBindJSON(&trace); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid trace data",
			},
		})
		return
	}

	// Store trace
	if err := h.traceRepo.Create(c.Request.Context(), projectID, &trace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to store trace",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":   "created",
		"trace_id": trace.TraceID,
	})
}

// UploadTracesBatch handles batch trace upload
func (h *TraceHandler) UploadTracesBatch(c *gin.Context) {
	projectID := c.Param("projectID")

	var req struct {
		Traces []regrada.Trace `json:"traces" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request body",
			},
		})
		return
	}

	if len(req.Traces) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "No traces provided",
			},
		})
		return
	}

	if len(req.Traces) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Maximum 100 traces per batch",
			},
		})
		return
	}

	// Store all traces
	if err := h.traceRepo.CreateBatch(c.Request.Context(), projectID, req.Traces); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to store traces",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "created",
		"count":  len(req.Traces),
	})
}

// ListTraces returns paginated list of traces
func (h *TraceHandler) ListTraces(c *gin.Context) {
	projectID := c.Param("projectID")

	// TODO: Add pagination and filtering
	traces, err := h.traceRepo.List(c.Request.Context(), projectID, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch traces",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"traces": traces,
		"count":  len(traces),
	})
}

// GetTrace returns a single trace
func (h *TraceHandler) GetTrace(c *gin.Context) {
	projectID := c.Param("projectID")
	traceID := c.Param("traceID")

	trace, err := h.traceRepo.Get(c.Request.Context(), projectID, traceID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Trace not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch trace",
			},
		})
		return
	}

	c.JSON(http.StatusOK, trace)
}
