package postgres

import (
	"time"

	"github.com/uptrace/bun"
)

// DBAPIKey represents an API key in the database
type DBAPIKey struct {
	bun.BaseModel `bun:"table:api_keys,alias:ak"`

	ID             string     `bun:"id,pk,type:uuid"`
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

	ID             string     `bun:"id,pk,type:uuid"`
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

	ID                string     `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	ProjectID         string     `bun:"project_id,type:uuid,notnull"`
	TraceID           string     `bun:"trace_id,notnull"`
	Timestamp         time.Time  `bun:"timestamp,notnull"`
	Provider          string     `bun:"provider,notnull"`
	Model             string     `bun:"model,notnull"`
	Environment       string     `bun:"environment"`
	GitSHA            string     `bun:"git_sha"`
	GitBranch         string     `bun:"git_branch"`
	RequestData       []byte     `bun:"request_data,type:jsonb,notnull"`
	ResponseData      []byte     `bun:"response_data,type:jsonb,notnull"`
	LatencyMS         int        `bun:"latency_ms"`
	TokensIn          int        `bun:"tokens_in"`
	TokensOut         int        `bun:"tokens_out"`
	RedactionApplied  []string   `bun:"redaction_applied,array"`
	Tags              []string   `bun:"tags,array"`
	CreatedAt         time.Time  `bun:"created_at,notnull,default:now()"`
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
}
