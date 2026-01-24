.PHONY: help dev db-up db-down db-migrate db-reset test build clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Start development environment
	docker-compose up -d

db-up: ## Start database services
	docker-compose up -d postgres redis

db-down: ## Stop database services
	docker-compose down

db-migrate: ## Run database migrations
	@echo "Running migrations..."
	@docker-compose exec -T postgres psql -U regrada -d regrada -f /docker-entrypoint-initdb.d/001_initial_schema.sql || \
	psql postgresql://regrada:regrada_dev@localhost:5432/regrada -f migrations/001_initial_schema.sql

db-reset: ## Reset database (WARNING: destroys data)
	docker-compose down -v
	docker-compose up -d postgres redis
	sleep 5
	$(MAKE) db-migrate

test: ## Run tests
	go test -v ./...

build-server: ## Build API server
	go build -o bin/server ./cmd/server

build-worker: ## Build worker
	go build -o bin/worker ./cmd/worker

build: build-server build-worker ## Build all binaries

clean: ## Clean build artifacts
	rm -rf bin/
	docker-compose down -v

.DEFAULT_GOAL := help
