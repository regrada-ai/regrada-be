package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	
	"github.com/regrada-ai/regrada-be/internal/api/handlers"
	apimiddleware "github.com/regrada-ai/regrada-be/internal/api/middleware"
	"github.com/regrada-ai/regrada-be/internal/storage/postgres"
)

func main() {
	// Load configuration from environment
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://regrada:regrada_dev@localhost:5432/regrada?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	ginMode := getEnv("GIN_MODE", "release")

	// Connect to PostgreSQL with Bun
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dbURL)))
	db := bun.NewDB(sqldb, pgdialect.New())
	
	// Add query hook for debugging (optional, can remove in production)
	if ginMode == "debug" {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✓ Connected to PostgreSQL")

	// Connect to Redis
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	redisClient := redis.NewClient(opt)
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("✓ Connected to Redis")

	// Initialize storage repositories
	orgRepo := postgres.NewOrganizationRepository(db)
	apiKeyRepo := postgres.NewAPIKeyRepository(db)
	projectRepo := postgres.NewProjectRepository(db)
	traceRepo := postgres.NewTraceRepository(db)
	testRunRepo := postgres.NewTestRunRepository(db)

	// Initialize handlers
	orgHandler := handlers.NewOrganizationHandler(orgRepo)
	projectHandler := handlers.NewProjectHandler(projectRepo)
	traceHandler := handlers.NewTraceHandler(traceRepo, projectRepo)
	testRunHandler := handlers.NewTestRunHandler(testRunRepo, projectRepo)
	healthHandler := handlers.NewHealthHandler(sqldb, redisClient)

	// Initialize middleware
	authMiddleware := apimiddleware.NewAuthMiddleware(apiKeyRepo, redisClient)
	rateLimitMiddleware := apimiddleware.NewRateLimitMiddleware(redisClient)

	// Setup Gin router
	gin.SetMode(ginMode)
	r := gin.Default()

	// Health check (no auth required)
	r.GET("/health", healthHandler.Health)

	// API routes
	v1 := r.Group("/v1")
	{
		// Public routes (no auth required for testing - in production, add auth)
		v1.POST("/organizations", orgHandler.CreateOrganization)
		v1.GET("/organizations/:orgID", orgHandler.GetOrganization)
		v1.POST("/projects", projectHandler.CreateProject)
		v1.GET("/projects", projectHandler.ListProjects)

		// Protected routes (require authentication)
		authenticated := v1.Group("")
		authenticated.Use(authMiddleware.Authenticate())
		authenticated.Use(rateLimitMiddleware.Limit())
		{
			// Project-specific routes
			projects := authenticated.Group("/projects/:projectID")
			{
				projects.GET("", projectHandler.GetProject)
				
				// Trace endpoints
				projects.POST("/traces", traceHandler.UploadTrace)
				projects.POST("/traces/batch", traceHandler.UploadTracesBatch)
				projects.GET("/traces", traceHandler.ListTraces)
				projects.GET("/traces/:traceID", traceHandler.GetTrace)

				// Test run endpoints
				projects.POST("/test-runs", testRunHandler.UploadTestRun)
				projects.GET("/test-runs", testRunHandler.ListTestRuns)
				projects.GET("/test-runs/:runID", testRunHandler.GetTestRun)
			}
		}
	}

	// Start server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		log.Printf("🚀 Server starting on port %s (mode: %s)", port, ginMode)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	db.Close()
	log.Println("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
