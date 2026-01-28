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
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/migrate"

	"github.com/regrada-ai/regrada-be/internal/api/handlers"
	apimiddleware "github.com/regrada-ai/regrada-be/internal/api/middleware"
	"github.com/regrada-ai/regrada-be/internal/auth"
	"github.com/regrada-ai/regrada-be/internal/email"
	"github.com/regrada-ai/regrada-be/internal/migrations"
	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/regrada-ai/regrada-be/internal/storage/postgres"
	"github.com/regrada-ai/regrada-be/internal/storage/s3"

	_ "github.com/regrada-ai/regrada-be/docs" // Swagger docs
)

// @title Regrada API
// @version 1.0
// @description API for LLM trace logging, testing, and regression detection

// @contact.name API Support
// @contact.email support@regrada.ai

// @license.name MIT

// @host localhost:8080
// @BasePath /v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name access_token

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
	s3Bucket := getEnv("S3_BUCKET", "")
	cloudFrontDomain := getEnv("CLOUDFRONT_DOMAIN", "")

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

	// Run database migrations
	ctx := context.Background()
	migrator := migrate.NewMigrator(db, migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize migrations: %v", err)
	}

	group, err := migrator.Migrate(ctx)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	if group.IsZero() {
		log.Println("✓ No new migrations to run")
	} else {
		log.Printf("✓ Migrated to %s\n", group)
	}

	// Connect to Redis
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	redisClient := redis.NewClient(opt)
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
	userRepo := postgres.NewUserRepository(db)
	memberRepo := postgres.NewOrganizationMemberRepository(db)
	inviteRepo := postgres.NewInviteRepository(db)

	// Initialize authentication service (Cognito)
	var authService auth.Service
	if cognitoUserPoolID != "" && cognitoClientID != "" {
		var err error
		authService, err = auth.NewCognitoService(awsRegion, cognitoUserPoolID, cognitoClientID, cognitoClientSecret)
		if err != nil {
			log.Fatalf("Failed to initialize authentication service: %v", err)
		}
		log.Println("✓ Authentication service initialized")
	} else {
		log.Println("⚠ Authentication service disabled (missing COGNITO_USER_POOL_ID or COGNITO_CLIENT_ID)")
	}

	// Initialize email service (optional)
	var emailService *email.Service
	fromEmail := getEnv("EMAIL_FROM_ADDRESS", "")
	fromName := getEnv("EMAIL_FROM_NAME", "Regrada")
	if fromEmail != "" {
		var err error
		emailService, err = email.NewService(awsRegion, fromEmail, fromName)
		if err != nil {
			log.Fatalf("Failed to initialize email service: %v", err)
		}
		log.Println("✓ Email service initialized")
	} else {
		log.Println("⚠ Email service disabled (missing EMAIL_FROM_ADDRESS)")
	}

	// Initialize file storage service (S3) (optional)
	var storageService storage.FileStorageService
	if s3Bucket != "" && cloudFrontDomain != "" {
		var err error
		storageService, err = s3.NewService(awsRegion, s3Bucket, cloudFrontDomain)
		if err != nil {
			log.Fatalf("Failed to initialize file storage service: %v", err)
		}
		log.Println("✓ File storage service initialized")
	} else {
		log.Println("⚠ File storage service disabled (missing S3_BUCKET or CLOUDFRONT_DOMAIN)")
	}

	// Initialize handlers
	orgHandler := handlers.NewOrganizationHandler(orgRepo, authService)
	apiKeyHandler := handlers.NewAPIKeyHandler(apiKeyRepo)
	projectHandler := handlers.NewProjectHandler(projectRepo)
	traceHandler := handlers.NewTraceHandler(traceRepo, projectRepo)
	testRunHandler := handlers.NewTestRunHandler(testRunRepo, projectRepo)
	healthHandler := handlers.NewHealthHandler(sqldb, redisClient)
	userHandler := handlers.NewUserHandler(userRepo, memberRepo, storageService)
	inviteHandler := handlers.NewInviteHandler(inviteRepo, userRepo, memberRepo)

	var authHandler *handlers.AuthHandler
	var emailHandler *handlers.EmailHandler
	if authService != nil {
		authHandler = handlers.NewAuthHandler(authService, secureCookies, userRepo, memberRepo, orgRepo, inviteRepo, storageService)
	}
	if emailService != nil {
		emailHandler = handlers.NewEmailHandler(emailService)
	}

	// Initialize middleware
	apiKeyAuthMiddleware := apimiddleware.NewAuthMiddleware(apiKeyRepo, redisClient)
	rateLimitMiddleware := apimiddleware.NewRateLimitMiddleware(redisClient)

	// Initialize cookie-based auth middleware if auth service is configured
	var cookieAuthMiddleware *apimiddleware.CookieAuthMiddleware
	if authService != nil {
		cookieAuthMiddleware = apimiddleware.NewCookieAuthMiddleware(authService)
		log.Println("✓ Cookie-based authentication enabled")
	}

	// Setup Gin router
	gin.SetMode(ginMode)
	r := gin.Default()
	allowedOrigins := strings.Split(getEnv("CORS_ALLOW_ORIGINS", "http://localhost:3000"), ",")
	r.Use(apimiddleware.NewCORSMiddleware(allowedOrigins))

	// Swagger documentation
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
		v1.GET("/invites/:token", inviteHandler.GetInvite)

		// Protected routes with cookie or API key auth
		eitherAuth := apimiddleware.NewEitherAuthMiddleware(cookieAuthMiddleware, apiKeyAuthMiddleware)
		protected := v1.Group("")
		protected.Use(eitherAuth.Authenticate())
		protected.Use(rateLimitMiddleware.Limit())
		{
			// Organization routes
			protected.GET("/organizations", orgHandler.ListOrganizations)
			protected.PUT("/organizations/:orgID", orgHandler.UpdateOrganization)
			protected.DELETE("/organizations/:orgID", orgHandler.DeleteOrganization)
			protected.POST("/organizations/:orgID/invite", orgHandler.InviteUser)
			protected.GET("/organizations/:orgID/users", userHandler.ListOrganizationUsers)
			protected.PUT("/organizations/:orgID/members/:userID", userHandler.UpdateOrganizationMemberRole)
			protected.DELETE("/organizations/:orgID/members/:userID", userHandler.RemoveOrganizationMember)

			// Invite routes
			protected.POST("/organizations/:orgID/invites", inviteHandler.CreateInvite)
			protected.GET("/organizations/:orgID/invites", inviteHandler.ListInvites)
			protected.POST("/invites/:token/accept", inviteHandler.AcceptInvite)
			protected.DELETE("/organizations/:orgID/invites/:inviteID", inviteHandler.RevokeInvite)

			// User routes
			protected.GET("/users/me", userHandler.GetCurrentUser)
			protected.GET("/users/:userID", userHandler.GetUser)
			protected.PUT("/users/:userID", userHandler.UpdateUser)
			protected.POST("/users/:userID/profile-picture", userHandler.UploadProfilePicture)
			protected.DELETE("/users/:userID/profile-picture", userHandler.DeleteProfilePicture)
			protected.DELETE("/users/:userID", userHandler.DeleteUser)

			// API Key routes
			protected.GET("/api-keys", apiKeyHandler.ListAPIKeys)
			protected.POST("/api-keys", apiKeyHandler.CreateAPIKey)
			protected.GET("/api-keys/:keyID", apiKeyHandler.GetAPIKey)
			protected.PUT("/api-keys/:keyID", apiKeyHandler.UpdateAPIKey)
			protected.POST("/api-keys/:keyID/revoke", apiKeyHandler.RevokeAPIKey)
			protected.DELETE("/api-keys/:keyID", apiKeyHandler.DeleteAPIKey)

			// Email route
			if emailHandler != nil {
				protected.POST("/email/send", emailHandler.SendEmail)
			}

			// Project routes
			protected.POST("/projects", projectHandler.CreateProject)
			protected.GET("/projects", projectHandler.ListProjects)

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
