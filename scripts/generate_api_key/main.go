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
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run ./scripts/generate_api_key <org_id> <name> <tier>")
		fmt.Println("Example: go run ./scripts/generate_api_key 123e4567-e89b-12d3-a456-426614174000 \"Dev API Key\" standard")
		os.Exit(1)
	}

	orgID := os.Args[1]
	name := os.Args[2]
	tier := os.Args[3]

	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://regrada:regrada_dev@localhost:5432/regrada?sslmode=disable"
	}

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dbURL)))
	db := bun.NewDB(sqldb, pgdialect.New())
	defer db.Close()

	ctx := context.Background()

	// Generate random API key
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		log.Fatalf("Failed to generate random bytes: %v", err)
	}

	apiKey := fmt.Sprintf("rg_live_%s", base64.RawURLEncoding.EncodeToString(randomBytes))

	// Hash the API key
	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])
	keyPrefix := apiKey[:16] // "rg_live_abc12345"

	// Get rate limit based on tier
	rateLimitRPM := 100
	switch tier {
	case "pro":
		rateLimitRPM = 500
	case "enterprise":
		rateLimitRPM = 2000
	}

	// Insert into database
	query := `
		INSERT INTO api_keys (
			organization_id, key_hash, key_prefix, name, tier,
			scopes, rate_limit_rpm
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?
		) RETURNING id
	`

	var id string
	err := db.NewRaw(query,
		orgID,
		keyHash,
		keyPrefix,
		name,
		tier,
		[]string{"traces:write", "tests:write", "projects:read"},
		rateLimitRPM,
	).Scan(ctx, &id)

	if err != nil {
		log.Fatalf("Failed to insert API key: %v", err)
	}

	fmt.Println("✅ API Key Generated Successfully!")
	fmt.Println("")
	fmt.Println("ID:", id)
	fmt.Println("Organization ID:", orgID)
	fmt.Println("Name:", name)
	fmt.Println("Tier:", tier)
	fmt.Println("Rate Limit:", rateLimitRPM, "RPM")
	fmt.Println("")
	fmt.Println("🔑 API Key (save this, it won't be shown again):")
	fmt.Println(apiKey)
	fmt.Println("")
	fmt.Println("Use this in the CLI with:")
	fmt.Printf("export REGRADA_API_KEY=%s\n", apiKey)
}
