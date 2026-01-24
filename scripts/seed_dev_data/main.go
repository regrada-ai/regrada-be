package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func main() {
	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://regrada:regrada_dev@localhost:5432/regrada?sslmode=disable"
	}

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dbURL)))
	db := bun.NewDB(sqldb, pgdialect.New())
	defer db.Close()

	ctx := context.Background()

	fmt.Println("🌱 Seeding development data...")

	// Create organization
	var orgID string
	err := db.NewRaw(`
		INSERT INTO organizations (name, slug, tier)
		VALUES (?, ?, ?)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, "Acme Corp", "acme", "pro").Scan(ctx, &orgID)

	if err != nil {
		log.Fatalf("Failed to create organization: %v", err)
	}
	fmt.Printf("✅ Organization created: %s (ID: %s)\n", "Acme Corp", orgID)

	// Create project
	var projectID string
	err = db.NewRaw(`
		INSERT INTO projects (organization_id, name, slug, github_owner, github_repo, default_branch)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (organization_id, slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, orgID, "My App", "my-app", "acme", "my-app", "main").Scan(ctx, &projectID)

	if err != nil {
		log.Fatalf("Failed to create project: %v", err)
	}
	fmt.Printf("✅ Project created: %s (ID: %s)\n", "My App", projectID)

	// Generate API key
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		log.Fatalf("Failed to generate random bytes: %v", err)
	}

	apiKey := fmt.Sprintf("rg_live_%s", base64.RawURLEncoding.EncodeToString(randomBytes))
	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])
	keyPrefix := apiKey[:16]

	var keyID string
	err = db.NewRaw(`
		INSERT INTO api_keys (
			organization_id, key_hash, key_prefix, name, tier,
			scopes, rate_limit_rpm
		) VALUES (?, ?, ?, ?, ?, ?::text[], ?)
		RETURNING id
	`, orgID, keyHash, keyPrefix, "Dev API Key", "pro", "{traces:write,tests:write,projects:read}", 500).Scan(ctx, &keyID)

	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	fmt.Println("")
	fmt.Println("🎉 Development data seeded successfully!")
	fmt.Println("")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Organization ID:", orgID)
	fmt.Println("Project ID:     ", projectID)
	fmt.Println("API Key ID:     ", keyID)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")
	fmt.Println("🔑 API Key (save this!):")
	fmt.Println(apiKey)
	fmt.Println("")
	fmt.Println("Use in CLI:")
	fmt.Printf("export REGRADA_API_KEY=%s\n", apiKey)
	fmt.Printf("export REGRADA_PROJECT_ID=%s\n", projectID)
	fmt.Println("")
	fmt.Println("Test the API:")
	fmt.Printf("curl http://localhost:8080/health\n")
	fmt.Printf("curl -H 'Authorization: Bearer %s' http://localhost:8080/v1/projects/%s/traces\n", apiKey, projectID)
}
