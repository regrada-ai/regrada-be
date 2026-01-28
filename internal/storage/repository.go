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
	GetByID(ctx context.Context, id string) (*APIKey, error)
	Create(ctx context.Context, apiKey *APIKey) error
	ListByOrganization(ctx context.Context, orgID string) ([]*APIKey, error)
	Update(ctx context.Context, apiKey *APIKey) error
	UpdateLastUsed(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
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
	ID            string
	Name          string
	Slug          string
	Tier          string
	GitHubOrgID   *int64
	GitHubOrgName string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// OrganizationRepository handles organization operations
type OrganizationRepository interface {
	Create(ctx context.Context, org *Organization) error
	Get(ctx context.Context, id string) (*Organization, error)
	Update(ctx context.Context, org *Organization) error
	Delete(ctx context.Context, id string) error
}

// UserRole represents the role a user can have in an organization
type UserRole string

const (
	UserRoleAdmin        UserRole = "admin"
	UserRoleUser         UserRole = "user"
	UserRoleReadonlyUser UserRole = "readonly-user"
)

// User represents a user
type User struct {
	ID        string
	Email     string
	IDPSub    string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserRepository handles user operations
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByIDPSub(ctx context.Context, idpSub string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
}

// OrganizationMember represents a user's membership in an organization
type OrganizationMember struct {
	ID             string
	OrganizationID string
	UserID         string
	Role           UserRole
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// OrganizationMemberRepository handles organization member operations
type OrganizationMemberRepository interface {
	Create(ctx context.Context, member *OrganizationMember) error
	GetByUserAndOrg(ctx context.Context, userID, orgID string) (*OrganizationMember, error)
	ListByUser(ctx context.Context, userID string) ([]*OrganizationMember, error)
	ListByOrganization(ctx context.Context, orgID string) ([]*OrganizationMember, error)
	UpdateRole(ctx context.Context, id string, role UserRole) error
	Delete(ctx context.Context, id string) error
}

// Invite represents an invitation to join an organization
type Invite struct {
	ID             string
	OrganizationID string
	Email          string
	Role           UserRole
	Token          string
	InvitedBy      *string
	AcceptedAt     *time.Time
	AcceptedBy     *string
	RevokedAt      *time.Time
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// InviteRepository handles invite operations
type InviteRepository interface {
	Create(ctx context.Context, invite *Invite) error
	GetByToken(ctx context.Context, token string) (*Invite, error)
	GetByEmailAndOrg(ctx context.Context, email, orgID string) (*Invite, error)
	ListByOrganization(ctx context.Context, orgID string) ([]*Invite, error)
	Accept(ctx context.Context, token, userID string) error
	Revoke(ctx context.Context, id string) error
}
