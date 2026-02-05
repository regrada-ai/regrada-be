// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package postgres

import (
	"time"

	"github.com/uptrace/bun"
)

// DBAPIKey represents an API key in the database
type DBAPIKey struct {
	bun.BaseModel `bun:"table:api_keys,alias:ak"`

	ID             string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	OrganizationID string     `bun:"organization_id,type:uuid,notnull"`
	KeyHash        string     `bun:"key_hash,notnull,unique"`
	KeyPrefix      string     `bun:"key_prefix,notnull"`
	Name           string     `bun:"name"`
	Tier           string     `bun:"tier,notnull"`
	Scopes         []string   `bun:"scopes,array"`
	RateLimitRPM   int        `bun:"rate_limit_rpm,notnull"`
	LastUsedAt     *time.Time `bun:"last_used_at"`
	ExpiresAt      *time.Time `bun:"expires_at"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:now()"`
	RevokedAt      *time.Time `bun:"revoked_at"`
}

// DBProject represents a project in the database
type DBProject struct {
	bun.BaseModel `bun:"table:projects,alias:p"`

	ID             string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	OrganizationID string     `bun:"organization_id,type:uuid,notnull"`
	Name           string     `bun:"name,notnull"`
	Slug           string     `bun:"slug,notnull"`
	GitRepoURL     string     `bun:"git_repo_url"`
	GitHubRepoID   *int64     `bun:"github_repo_id"`
	GitHubOwner    string     `bun:"github_owner"`
	GitHubRepo     string     `bun:"github_repo"`
	DefaultBranch  string     `bun:"default_branch"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt      time.Time  `bun:"updated_at,notnull,default:now()"`
	DeletedAt      *time.Time `bun:"deleted_at,soft_delete"`
}

// DBTrace represents a trace in the database
type DBTrace struct {
	bun.BaseModel `bun:"table:traces,alias:t"`

	ID               string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	ProjectID        string     `bun:"project_id,type:uuid,notnull"`
	TraceID          string     `bun:"trace_id,notnull"`
	Timestamp        time.Time  `bun:"timestamp,notnull"`
	Provider         string     `bun:"provider,notnull"`
	Model            string     `bun:"model,notnull"`
	Environment      string     `bun:"environment"`
	GitSHA           string     `bun:"git_sha"`
	GitBranch        string     `bun:"git_branch"`
	RequestData      []byte     `bun:"request_data,type:jsonb,notnull"`
	ResponseData     []byte     `bun:"response_data,type:jsonb,notnull"`
	LatencyMS        int        `bun:"latency_ms"`
	TokensIn         int        `bun:"tokens_in"`
	TokensOut        int        `bun:"tokens_out"`
	RedactionApplied []string   `bun:"redaction_applied,array"`
	Tags             []string   `bun:"tags,array"`
	CreatedAt        time.Time  `bun:"created_at,notnull,default:now()"`
	DeletedAt        *time.Time `bun:"deleted_at,soft_delete"`
}

// DBTestRun represents a test run in the database
type DBTestRun struct {
	bun.BaseModel `bun:"table:test_runs,alias:tr"`

	ID               string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	ProjectID        string     `bun:"project_id,type:uuid,notnull"`
	RunID            string     `bun:"run_id,notnull"`
	Timestamp        time.Time  `bun:"timestamp,notnull"`
	GitSHA           string     `bun:"git_sha,notnull"`
	GitBranch        string     `bun:"git_branch"`
	GitCommitMessage string     `bun:"git_commit_message"`
	CIProvider       string     `bun:"ci_provider"`
	CIPRNumber       int        `bun:"ci_pr_number"`
	TotalCases       int        `bun:"total_cases,notnull"`
	PassedCases      int        `bun:"passed_cases,notnull"`
	WarnedCases      int        `bun:"warned_cases,notnull"`
	FailedCases      int        `bun:"failed_cases,notnull"`
	Results          []byte     `bun:"results,type:jsonb,notnull"`
	Violations       []byte     `bun:"violations,type:jsonb,notnull"`
	Status           string     `bun:"status,notnull"`
	CreatedAt        time.Time  `bun:"created_at,notnull,default:now()"`
	CompletedAt      *time.Time `bun:"completed_at"`
	DeletedAt        *time.Time `bun:"deleted_at,soft_delete"`
}

// DBOrganization represents an organization in the database
type DBOrganization struct {
	bun.BaseModel `bun:"table:organizations,alias:o"`

	ID                  string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Name                string     `bun:"name,notnull"`
	Slug                string     `bun:"slug,notnull,unique"`
	Tier                string     `bun:"tier,notnull"`
	GitHubOrgID         *int64     `bun:"github_org_id"`
	GitHubOrgName       string     `bun:"github_org_name"`
	MonthlyRequestLimit int64      `bun:"monthly_request_limit,notnull,default:50000"`
	MonthlyRequestCount int64      `bun:"monthly_request_count,notnull,default:0"`
	UsageResetAt        time.Time  `bun:"usage_reset_at,notnull"`
	CreatedAt           time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt           time.Time  `bun:"updated_at,notnull,default:now()"`
	DeletedAt           *time.Time `bun:"deleted_at,soft_delete"`
}

// UserRole represents the role a user can have in an organization
type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
	UserRoleViewer UserRole = "viewer"
)

// DBUser represents a user in the database
type DBUser struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID             string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Email          string     `bun:"email,notnull,unique"`
	IDPSub         string     `bun:"idp_sub,notnull,unique"` // Cognito subject identifier
	Name           string     `bun:"name"`
	ProfilePicture string     `bun:"profile_picture"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt      time.Time  `bun:"updated_at,notnull,default:now()"`
	DeletedAt      *time.Time `bun:"deleted_at,soft_delete"`
}

// DBOrganizationMember represents a user's membership in an organization
type DBOrganizationMember struct {
	bun.BaseModel `bun:"table:organization_members,alias:om"`

	ID             string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	OrganizationID string     `bun:"organization_id,type:uuid,notnull"`
	UserID         string     `bun:"user_id,type:uuid,notnull"`
	Role           UserRole   `bun:"role,notnull,default:'member'"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt      time.Time  `bun:"updated_at,notnull,default:now()"`
	DeletedAt      *time.Time `bun:"deleted_at,soft_delete"`
}

// DBInvite represents an invitation to join an organization
type DBInvite struct {
	bun.BaseModel `bun:"table:invites,alias:inv"`

	ID             string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	OrganizationID string     `bun:"organization_id,type:uuid,notnull"`
	Email          string     `bun:"email,notnull"`
	Role           UserRole   `bun:"role,notnull,default:'member'"`
	Token          string     `bun:"token,notnull,unique"`
	InvitedBy      *string    `bun:"invited_by,type:uuid"`
	AcceptedAt     *time.Time `bun:"accepted_at"`
	AcceptedBy     *string    `bun:"accepted_by,type:uuid"`
	RevokedAt      *time.Time `bun:"revoked_at"`
	ExpiresAt      time.Time  `bun:"expires_at,notnull"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt      time.Time  `bun:"updated_at,notnull,default:now()"`
}
