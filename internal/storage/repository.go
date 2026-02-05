// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package storage

import (
	"context"
	"errors"
	"time"

	"github.com/regrada-ai/regrada-be/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// APIKey represents an API key in the database
type APIKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	KeyHash        string     `json:"-"`
	KeyPrefix      string     `json:"key_prefix"`
	Name           string     `json:"name"`
	Tier           string     `json:"tier"`
	Scopes         []string   `json:"scopes"`
	RateLimitRPM   int        `json:"rate_limit_rpm"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
}

// APIKeyRepository handles API key operations
type APIKeyRepository interface {
	GetByHash(ctx context.Context, keyHash string) (*APIKey, error)
	GetByID(ctx context.Context, id string) (*APIKey, error)
	Create(ctx context.Context, apiKey *APIKey) error
	ListByOrganization(ctx context.Context, orgID string) ([]*APIKey, error)
	Update(ctx context.Context, apiKey *APIKey) error
	UpdateLastUsed(ctx context.Context, id string) error
	UpdateTierByOrganization(ctx context.Context, orgID, tier string, rateLimitRPM int) error
	Revoke(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// ProjectRepository handles project operations
type ProjectRepository interface {
	Create(ctx context.Context, project *Project) error
	Get(ctx context.Context, id string) (*Project, error)
	ListByOrganization(ctx context.Context, orgID string) ([]*Project, error)
	Update(ctx context.Context, project *Project) error
	Delete(ctx context.Context, id string) error
}

// Project represents a project
type Project struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
}

// TraceRepository handles trace storage operations
type TraceRepository interface {
	Create(ctx context.Context, projectID string, trace *domain.Trace) error
	CreateBatch(ctx context.Context, projectID string, traces []domain.Trace) error
	Get(ctx context.Context, projectID, traceID string) (*domain.Trace, error)
	List(ctx context.Context, projectID string, limit, offset int) ([]*domain.Trace, error)
	Delete(ctx context.Context, projectID, traceID string) error
}

// TestRunRepository handles test run storage operations
type TestRunRepository interface {
	Create(ctx context.Context, projectID string, testRun *domain.TestRun) error
	Get(ctx context.Context, projectID, runID string) (*domain.TestRun, error)
	List(ctx context.Context, projectID string, limit, offset int) ([]*domain.TestRun, error)
	Delete(ctx context.Context, projectID, runID string) error
}

// Organization represents an organization
type Organization struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Slug                string    `json:"slug"`
	Tier                string    `json:"tier"`
	GitHubOrgID         *int64    `json:"github_org_id,omitempty"`
	GitHubOrgName       string    `json:"github_org_name,omitempty"`
	MonthlyRequestLimit int64     `json:"monthly_request_limit"`
	MonthlyRequestCount int64     `json:"monthly_request_count"`
	UsageResetAt        time.Time `json:"usage_reset_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// OrganizationRepository handles organization operations
type OrganizationRepository interface {
	Create(ctx context.Context, org *Organization) error
	Get(ctx context.Context, id string) (*Organization, error)
	GetByUser(ctx context.Context, userID string) ([]*Organization, error)
	Update(ctx context.Context, org *Organization) error
	Delete(ctx context.Context, id string) error
	IncrementRequestCount(ctx context.Context, id string) (*Organization, error)
	ResetMonthlyUsage(ctx context.Context, id string) error
}

// UserRole represents the role a user can have in an organization
type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
	UserRoleViewer UserRole = "viewer"
)

// User represents a user
type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	IDPSub         string    `json:"-"`
	Name           string    `json:"name"`
	ProfilePicture string    `json:"profile_picture,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	UserID         string    `json:"user_id"`
	Role           UserRole  `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	Email          string     `json:"email"`
	Role           UserRole   `json:"role"`
	Token          string     `json:"token"`
	InvitedBy      *string    `json:"invited_by,omitempty"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	AcceptedBy     *string    `json:"accepted_by,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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
