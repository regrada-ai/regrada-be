package storage

import (
	"context"
	"errors"
	"time"

	"github.com/regrada-ai/regrada-be/pkg/regrada"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// APIKey represents an API key in the database
type APIKey struct {
	ID             string
	OrganizationID string
	KeyHash        string
	KeyPrefix      string
	Name           string
	Tier           string
	Scopes         []string
	RateLimitRPM   int
	LastUsedAt     *time.Time
	ExpiresAt      *time.Time
	CreatedAt      time.Time
	RevokedAt      *time.Time
}

// APIKeyRepository handles API key operations
type APIKeyRepository interface {
	GetByHash(ctx context.Context, keyHash string) (*APIKey, error)
	Create(ctx context.Context, apiKey *APIKey) error
	ListByOrganization(ctx context.Context, orgID string) ([]*APIKey, error)
	UpdateLastUsed(ctx context.Context, id string) error
}

// ProjectRepository handles project operations
// ProjectRepository handles project operations
type ProjectRepository interface {
	Create(ctx context.Context, project *Project) error
	Get(ctx context.Context, id string) (*Project, error)
	ListByOrganization(ctx context.Context, orgID string) ([]*Project, error)
}

// Project represents a project
type Project struct {
	ID             string
	OrganizationID string
	Name           string
	Slug           string
}

// TraceRepository handles trace storage operations
type TraceRepository interface {
	Create(ctx context.Context, projectID string, trace *regrada.Trace) error
	CreateBatch(ctx context.Context, projectID string, traces []regrada.Trace) error
	Get(ctx context.Context, projectID, traceID string) (*regrada.Trace, error)
	List(ctx context.Context, projectID string, limit, offset int) ([]*regrada.Trace, error)
}

// TestRunRepository handles test run storage operations
type TestRunRepository interface {
	Create(ctx context.Context, projectID string, testRun *regrada.TestRun) error
	Get(ctx context.Context, projectID, runID string) (*regrada.TestRun, error)
	List(ctx context.Context, projectID string, limit, offset int) ([]*regrada.TestRun, error)
}

// Organization represents an organization
type Organization struct {
	ID             string
	Name           string
	Slug           string
	Tier           string
	GitHubOrgID    *int64
	GitHubOrgName  string
}

// OrganizationRepository handles organization operations
type OrganizationRepository interface {
	Create(ctx context.Context, org *Organization) error
	Get(ctx context.Context, id string) (*Organization, error)
}
