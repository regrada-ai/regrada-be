.PHONY: help dev db-up db-down db-migrate db-reset db-seed test build clean run

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Start development environment (all services)
	docker compose up -d

db-up: ## Start database services only
	docker compose up -d postgres redis

db-down: ## Stop all services
	docker compose down

db-migrate: ## Run database migrations
	@echo "Running migrations..."
	@docker compose exec -T postgres psql -U regrada -d regrada -f /docker-entrypoint-initdb.d/001_initial_schema.sql 2>/dev/null || \
	psql postgresql://regrada:regrada_dev@localhost:5432/regrada -f migrations/001_initial_schema.sql

db-reset: ## Reset database (WARNING: destroys data)
	docker compose down -v
	docker compose up -d postgres redis
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5
	$(MAKE) db-migrate

db-seed: ## Seed development data
	@echo "Seeding development data..."
	go run scripts/seed_dev_data.go

generate-key: ## Generate a new API key (usage: make generate-key ORG_ID=xxx NAME="key name" TIER=standard)
	@if [ -z "$(ORG_ID)" ]; then echo "Error: ORG_ID required. Usage: make generate-key ORG_ID=xxx NAME=\"key name\" TIER=standard"; exit 1; fi
	@if [ -z "$(NAME)" ]; then echo "Error: NAME required"; exit 1; fi
	@if [ -z "$(TIER)" ]; then echo "Error: TIER required (standard/pro/enterprise)"; exit 1; fi
	go run scripts/generate_api_key.go $(ORG_ID) "$(NAME)" $(TIER)

test: ## Run tests
	go test -v ./...

build-server: ## Build API server
	go build -o bin/server ./cmd/server

build: build-server ## Build all binaries

run: ## Run the server locally
	go run cmd/server/main.go

clean: ## Clean build artifacts and stop services
	rm -rf bin/
	docker compose down -v

.DEFAULT_GOAL := help
