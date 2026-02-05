.PHONY: help install-tools dev build test clean docs run docker-up docker-down migrate

# Default target
help:
	@echo "Available targets:"
	@echo "  make install-tools  - Install development tools (swag, etc.)"
	@echo "  make docs          - Generate Swagger documentation"
	@echo "  make dev           - Run development server with auto-docs generation"
	@echo "  make run           - Run server without docs generation"
	@echo "  make build         - Build the server binary"
	@echo "  make test          - Run tests"
	@echo "  make docker-up     - Start Docker services (postgres, redis)"
	@echo "  make docker-down   - Stop Docker services"
	@echo "  make clean         - Clean build artifacts and generated docs"

# Install development tools
install-tools:
	@echo "Installing swag..."
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "✓ Tools installed"

# Generate Swagger docs
docs:
	@echo "Generating Swagger documentation..."
	@$(shell go env GOPATH)/bin/swag init -g cmd/server.go -o docs
	@echo "✓ Swagger docs generated"

# Run development server with auto-docs generation
dev: docs docker-up
	@echo "Starting development server..."
	@bash -c 'set -a && source .env && set +a && go run ./cmd/server.go'

# Run server without docs generation
run:
	@bash -c 'set -a && source .env && set +a && go run ./cmd/server.go'

# Build the server
build: docs
	@echo "Building server..."
	CGO_ENABLED=0 go build -o bin/server ./cmd/server.go
	@echo "✓ Server built at bin/server"

# Run tests
test:
	go test -v ./...

# Start Docker services
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d postgres redis
	@echo "✓ Docker services running"

# Stop Docker services
docker-down:
	docker-compose down

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf docs/
	@echo "✓ Cleaned"
