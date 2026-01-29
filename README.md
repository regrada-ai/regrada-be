# Regrada Backend (regrada-be)

Backend service for Regrada - LLM testing and evaluation platform.

## Overview

This backend provides:
- API key authentication with tiered plans (Standard/Pro/Enterprise)
- Trace and test result storage
- Dashboard APIs for visualization
- GitHub integration (PR comments, regression detection)
- Usage tracking and billing

## Tech Stack

- **Language**: Go 1.23+
- **Database**: PostgreSQL 15+ (with JSONB support)
- **Cache/Queue**: Redis 7+
- **Framework**: Chi router
- **Deployment**: Docker + AWS ECS/EKS

## Project Structure

```
regrada-be/
├── cmd/
│   ├── server/          # API server entrypoint
│   ├── worker/          # Background job processor
│   └── migrate/         # Database migration tool
├── internal/
│   ├── api/             # HTTP handlers and middleware
│   ├── domain/          # Business logic entities
│   ├── storage/         # Database repositories
│   ├── services/        # Business services
│   ├── jobs/            # Background jobs
│   ├── cache/           # Redis cache
│   ├── queue/           # Message queue
│   └── github/          # GitHub integration
├── pkg/
│   └── regrada/         # Shared types (importable by CLI)
├── migrations/          # Database migrations
├── docker/              # Dockerfiles
└── docker-compose.yml   # Local development setup
```

## Getting Started

### Prerequisites

- Go 1.23+
- Docker and Docker Compose
- PostgreSQL 15+ (or use Docker Compose)
- Redis 7+ (or use Docker Compose)

### Local Development

1. Start dependencies:
```bash
docker-compose up -d
```

2. Run database migrations:
```bash
psql postgresql://regrada:regrada_dev@localhost:5432/regrada -f migrations/001_initial_schema.sql
```

3. Run the server:
```bash
go run cmd/server/main.go
```

4. Access the API documentation at http://localhost:8080/docs

### Updating API Documentation

When you add or modify API endpoints, regenerate the Swagger documentation:

```bash
# Install swag tool (one time)
go install github.com/swaggo/swag/cmd/swag@latest

# Regenerate docs
swag init -g cmd/server/main.go -o docs
```

## API Documentation

API base URL: `https://api.regrada.com/v1`

### Interactive API Docs

When running locally, access the interactive Swagger UI documentation at:
```
http://localhost:8080/docs
```

The Swagger UI provides:
- Complete API reference with request/response examples
- Interactive testing of endpoints
- Authentication configuration
- Schema definitions

### Authentication

All requests require an API key:
```
Authorization: Bearer rg_live_<key>
```

### Key Endpoints

- `POST /v1/projects/:id/traces` - Upload a single trace
- `POST /v1/projects/:id/traces/batch` - Upload traces in batch (max 100)
- `GET /v1/projects/:id/traces` - List traces
- `GET /v1/projects/:id/traces/:traceID` - Get a specific trace
- `POST /v1/projects/:id/test-runs` - Upload test results
- `GET /v1/projects/:id/test-runs` - List test runs
- `GET /v1/projects/:id/test-runs/:runID` - Get a specific test run
- `GET /health` - Health check endpoint

## Shared Types Package

The `pkg/regrada/` package contains shared types used by both the backend and CLI:

```go
import "github.com/regrada-ai/regrada-be/pkg/regrada"

client := regrada.NewClient(apiKey, projectID)
err := client.UploadTraces(ctx, traces)
```

## Development Roadmap

- [x] Phase 1: Core backend (Weeks 1-4)
  - [x] Repository setup
  - [x] Database schema
  - [x] Shared types package
  - [ ] API authentication
  - [ ] Trace/test upload endpoints
- [ ] Phase 2: Dashboard (Weeks 5-8)
- [ ] Phase 3: GitHub integration (Weeks 9-12)
- [ ] Phase 4: Production (Weeks 13-16)

## License

This project is proprietary software owned by Regrada, Inc.
All rights reserved.

No use, reproduction, modification, or distribution is permitted
without explicit written authorization from Regrada, Inc.
