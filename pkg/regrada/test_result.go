package regrada

import (
	"encoding/json"
	"time"
)

// TestRun represents a complete test execution
type TestRun struct {
	RunID            string       `json:"run_id"`
	Timestamp        time.Time    `json:"timestamp"`
	GitSHA           string       `json:"git_sha"`
	GitBranch        string       `json:"git_branch,omitempty"`
	GitCommitMessage string       `json:"git_commit_message,omitempty"`
	CIProvider       string       `json:"ci_provider,omitempty"`
	CIBuildID        string       `json:"ci_build_id,omitempty"`
	CIBuildURL       string       `json:"ci_build_url,omitempty"`
	CIPRNumber       int          `json:"ci_pr_number,omitempty"`
	Config           interface{}  `json:"config,omitempty"` // regrada.yml snapshot
	Results          []CaseResult `json:"results"`
	Violations       []Violation  `json:"violations"`
	TotalCases       int          `json:"total_cases"`
	PassedCases      int          `json:"passed_cases"`
	WarnedCases      int          `json:"warned_cases"`
	FailedCases      int          `json:"failed_cases"`
	Status           string       `json:"status,omitempty"`
}

// CaseResult represents the result of running a single test case
type CaseResult struct {
	CaseID     string      `json:"case_id"`
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	Runs       []RunResult `json:"runs"`
	Aggregates Aggregates  `json:"aggregates"`
}

// RunResult represents a single run of a test case
type RunResult struct {
	RunID      int             `json:"run_id"`
	Pass       bool            `json:"pass"`
	OutputText string          `json:"output_text,omitempty"`
	JSON       json.RawMessage `json:"json,omitempty"`
	Metrics    RunMetrics      `json:"metrics"`
	Error      string          `json:"error,omitempty"`
}

// RunMetrics contains metrics for a single test run
type RunMetrics struct {
	LatencyMS int  `json:"latency_ms"`
	Refused   bool `json:"refused"`
	JSONValid bool `json:"json_valid"`
}

// Aggregates contains aggregated metrics across multiple runs
type Aggregates struct {
	PassRate      float64 `json:"pass_rate"`
	LatencyP95MS  int     `json:"latency_p95_ms"`
	RefusalRate   float64 `json:"refusal_rate"`
	JSONValidRate float64 `json:"json_valid_rate"`
}

// Violation represents a policy violation
type Violation struct {
	PolicyID string `json:"policy_id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}
