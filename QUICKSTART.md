# Regrada Backend - Quick Start Guide

## 🚀 Getting Started (3 steps)

### 1. Start all services
```bash
make dev
```

This starts:
- PostgreSQL (port 5432)
- Redis (port 6379)
- Backend API (port 8080)

### 2. Run migrations
```bash
make db-migrate
```

### 3. Seed development data
```bash
make db-seed
```

This creates:
- Organization: "Acme Corp" (tier: pro)
- Project: "My App"
- API Key with 500 RPM rate limit

The seed script will print your API key and project ID.

## 📝 Example Output

```
🎉 Development data seeded successfully!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Organization ID: 123e4567-e89b-12d3-a456-426614174000
Project ID:      456e7890-e89b-12d3-a456-426614174000
API Key ID:      789e0123-e89b-12d3-a456-426614174000
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔑 API Key (save this!):
rg_live_abc123def456...

Use in CLI:
export REGRADA_API_KEY=rg_live_abc123def456...
export REGRADA_PROJECT_ID=456e7890-e89b-12d3-a456-426614174000
```

## 🧪 Test the API

### Health Check
```bash
curl http://localhost:8080/health
```

### Upload a Trace
```bash
curl -X POST http://localhost:8080/v1/projects/YOUR_PROJECT_ID/traces \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "test_trace_001",
    "timestamp": "2026-01-24T12:00:00Z",
    "provider": "openai",
    "model": "gpt-4",
    "request": {
      "messages": [
        {"role": "user", "content": "Hello, world!"}
      ]
    },
    "response": {
      "assistant_text": "Hi! How can I help you today?"
    },
    "metrics": {
      "latency_ms": 450,
      "tokens_in": 10,
      "tokens_out": 15
    }
  }'
```

### Upload Traces (Batch)
```bash
curl -X POST http://localhost:8080/v1/projects/YOUR_PROJECT_ID/traces/batch \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "traces": [
      {
        "trace_id": "batch_001",
        "timestamp": "2026-01-24T12:00:00Z",
        "provider": "openai",
        "model": "gpt-4",
        "request": {"messages": [{"role": "user", "content": "Test"}]},
        "response": {"assistant_text": "Response"},
        "metrics": {"latency_ms": 300}
      }
    ]
  }'
```

### Upload Test Run
```bash
curl -X POST http://localhost:8080/v1/projects/YOUR_PROJECT_ID/test-runs \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "run_id": "test_run_001",
    "timestamp": "2026-01-24T12:00:00Z",
    "git_sha": "abc123def456",
    "git_branch": "main",
    "total_cases": 5,
    "passed_cases": 4,
    "warned_cases": 0,
    "failed_cases": 1,
    "results": [],
    "violations": []
  }'
```

### List Traces
```bash
curl http://localhost:8080/v1/projects/YOUR_PROJECT_ID/traces \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### List Test Runs
```bash
curl http://localhost:8080/v1/projects/YOUR_PROJECT_ID/test-runs \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## 🛠️ Useful Commands

```bash
# Rebuild server
make build

# Run server locally (without Docker)
make run

# Generate a new API key
make generate-key ORG_ID=xxx NAME="Production Key" TIER=enterprise

# Reset database (WARNING: deletes all data)
make db-reset

# Stop all services
make db-down

# Clean everything
make clean
```

## 📊 What's Running

| Service | Port | URL |
|---------|------|-----|
| Backend API | 8080 | http://localhost:8080 |
| PostgreSQL | 5432 | postgres://regrada:regrada_dev@localhost:5432/regrada |
| Redis | 6379 | redis://localhost:6379 |

## 🔍 Debugging

### Check logs
```bash
docker compose logs -f server     # Backend logs
docker compose logs -f postgres   # Database logs
docker compose logs -f redis      # Redis logs
```

### Connect to database
```bash
psql postgres://regrada:regrada_dev@localhost:5432/regrada
```

### Check Redis
```bash
redis-cli
```

## 📈 Rate Limits

| Tier | Requests per Minute |
|------|---------------------|
| Standard | 100 |
| Pro | 500 |
| Enterprise | 2000 |

Rate limit headers are included in all responses:
```
X-RateLimit-Limit: 500
X-RateLimit-Remaining: 499
X-RateLimit-Reset: 1706097660
```

## 🎯 Next Steps

Your backend is ready! Now you can:

1. **Integrate with CLI**: Update the regrada CLI to send traces/results to this API
2. **Build Dashboard**: Create a frontend to visualize the data
3. **Add GitHub Integration**: Implement PR comments and regression detection

All the infrastructure is in place and tested! 🎉
