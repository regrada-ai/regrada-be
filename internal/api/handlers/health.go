package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db          *sql.DB
	redisClient *redis.Client
}

func NewHealthHandler(db *sql.DB, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:          db,
		redisClient: redisClient,
	}
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	status := gin.H{
		"status": "ok",
		"checks": gin.H{},
	}

	// Check database
	if err := h.db.PingContext(ctx); err != nil {
		status["status"] = "error"
		status["checks"].(gin.H)["database"] = "down"
	} else {
		status["checks"].(gin.H)["database"] = "up"
	}

	// Check Redis
	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		status["status"] = "error"
		status["checks"].(gin.H)["redis"] = "down"
	} else {
		status["checks"].(gin.H)["redis"] = "up"
	}

	if status["status"] == "error" {
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}
