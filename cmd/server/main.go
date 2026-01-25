package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	"github.com/regrada-ai/regrada-be/internal/auth"
	"github.com/regrada-ai/regrada-be/internal/storage/postgres"
)

func main() {
	// Load configuration from environment
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://regrada:regrada_dev@localhost:5432/regrada?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	ginMode := getEnv("GIN_MODE", "release")
	awsRegion := getEnv("AWS_REGION", "us-east-1")
	cognitoUserPoolID := getEnv("COGNITO_USER_POOL_ID", "")
	cognitoClientID := getEnv("COGNITO_CLIENT_ID", "")
	cognitoClientSecret := getEnv("COGNITO_CLIENT_SECRET", "")
	secureCookies := getEnv("SECURE_COOKIES", "false") == "true"

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

	// Initialize Cognito service
	var cognitoService *auth.CognitoService
	if cognitoUserPoolID != "" && cognitoClientID != "" {
		var err error
		cognitoService, err = auth.NewCognitoService(awsRegion, cognitoUserPoolID, cognitoClientID, cognitoClientSecret)
		if err != nil {
			log.Fatalf("Failed to initialize Cognito service: %v", err)
		}
		log.Println("✓ Cognito service initialized")
	} else {
		log.Println("⚠ Cognito service disabled (missing COGNITO_USER_POOL_ID or COGNITO_CLIENT_ID)")
	}

	// Initialize handlers
	orgHandler := handlers.NewOrganizationHandler(orgRepo, cognitoService)
	apiKeyHandler := handlers.NewAPIKeyHandler(apiKeyRepo)
	projectHandler := handlers.NewProjectHandler(projectRepo)
	traceHandler := handlers.NewTraceHandler(traceRepo, projectRepo)
	testRunHandler := handlers.NewTestRunHandler(testRunRepo, projectRepo)
	healthHandler := handlers.NewHealthHandler(sqldb, redisClient)

	var authHandler *handlers.AuthHandler
	if cognitoService != nil {
		authHandler = handlers.NewAuthHandler(cognitoService, secureCookies)
	}

	// Initialize middleware
	apiKeyAuthMiddleware := apimiddleware.NewAuthMiddleware(apiKeyRepo, redisClient)
	rateLimitMiddleware := apimiddleware.NewRateLimitMiddleware(redisClient)

	// Initialize cookie-based auth middleware if Cognito is configured
	var cookieAuthMiddleware *apimiddleware.CookieAuthMiddleware
	if cognitoService != nil {
		cookieAuthMiddleware = apimiddleware.NewCookieAuthMiddleware(cognitoService)
		log.Println("✓ Cookie-based authentication enabled")
	}

	// Setup Gin router
	gin.SetMode(ginMode)
	r := gin.Default()
	allowedOrigins := strings.Split(getEnv("CORS_ALLOW_ORIGINS", "http://localhost:3000"), ",")
	r.Use(apimiddleware.NewCORSMiddleware(allowedOrigins))

	// Health check (no auth required)
	r.GET("/health", healthHandler.Health)

	// API routes
	v1 := r.Group("/v1")
	{
		// Authentication routes (no auth required)
		if authHandler != nil {
			auth := v1.Group("/auth")
			{
				auth.POST("/signup", authHandler.SignUp)
				auth.POST("/confirm", authHandler.ConfirmSignUp)
				auth.POST("/signin", authHandler.SignIn)
				auth.POST("/signout", authHandler.SignOut)
				auth.POST("/refresh", authHandler.RefreshToken)
				auth.GET("/me", authHandler.Me)
			}
		}

		// Public routes (no auth required)
		v1.POST("/organizations", orgHandler.CreateOrganization)
		v1.GET("/organizations/:orgID", orgHandler.GetOrganization)

		// Protected routes with cookie or API key auth
		eitherAuth := apimiddleware.NewEitherAuthMiddleware(cookieAuthMiddleware, apiKeyAuthMiddleware)
		protected := v1.Group("")
		protected.Use(eitherAuth.Authenticate())
		protected.Use(rateLimitMiddleware.Limit())
		{
			protected.POST("/organizations/:orgID/invite", orgHandler.InviteUser)
			protected.POST("/projects", projectHandler.CreateProject)
			protected.GET("/projects", projectHandler.ListProjects)
			protected.GET("/api-keys", apiKeyHandler.ListAPIKeys)
			protected.POST("/api-keys", apiKeyHandler.CreateAPIKey)

			projects := protected.Group("/projects/:projectID")
			{
				projects.GET("", projectHandler.GetProject)
				projects.POST("/traces", traceHandler.UploadTrace)
				projects.POST("/traces/batch", traceHandler.UploadTracesBatch)
				projects.GET("/traces", traceHandler.ListTraces)
				projects.GET("/traces/:traceID", traceHandler.GetTrace)
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
