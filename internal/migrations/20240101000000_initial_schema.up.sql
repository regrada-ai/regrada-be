-- Organizations table
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    tier VARCHAR(50) NOT NULL DEFAULT 'standard',
    github_org_id BIGINT UNIQUE,
    github_org_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CHECK (tier IN ('standard', 'pro', 'enterprise'))
);

CREATE INDEX idx_organizations_slug ON organizations(slug);
CREATE INDEX idx_organizations_github_org_id ON organizations(github_org_id) WHERE github_org_id IS NOT NULL;

-- API Keys table
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash VARCHAR(128) NOT NULL UNIQUE,
    key_prefix VARCHAR(16) NOT NULL,
    name VARCHAR(255),
    tier VARCHAR(50) NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    rate_limit_rpm INTEGER NOT NULL DEFAULT 100,
    monthly_trace_limit INTEGER,
    monthly_test_limit INTEGER,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    CHECK (tier IN ('standard', 'pro', 'enterprise'))
);

CREATE INDEX idx_api_keys_organization_id ON api_keys(organization_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_revoked ON api_keys(revoked_at) WHERE revoked_at IS NULL;

-- Projects table
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    git_repo_url VARCHAR(500),
    github_repo_id BIGINT UNIQUE,
    github_owner VARCHAR(255),
    github_repo VARCHAR(255),
    default_branch VARCHAR(255) DEFAULT 'main',
    settings JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(organization_id, slug)
);

CREATE INDEX idx_projects_organization_id ON projects(organization_id);
CREATE INDEX idx_projects_github_repo_id ON projects(github_repo_id) WHERE github_repo_id IS NOT NULL;
CREATE INDEX idx_projects_github_owner_repo ON projects(github_owner, github_repo);

-- Traces table
CREATE TABLE traces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    trace_id VARCHAR(255) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(255) NOT NULL,
    environment VARCHAR(50),
    git_sha VARCHAR(40),
    git_branch VARCHAR(255),
    request_data JSONB NOT NULL,
    response_data JSONB NOT NULL,
    latency_ms INTEGER,
    tokens_in INTEGER,
    tokens_out INTEGER,
    redaction_applied TEXT[],
    tags TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, trace_id)
);

CREATE INDEX idx_traces_project_id ON traces(project_id);
CREATE INDEX idx_traces_timestamp ON traces(timestamp DESC);
CREATE INDEX idx_traces_git_sha ON traces(git_sha) WHERE git_sha IS NOT NULL;
CREATE INDEX idx_traces_provider_model ON traces(provider, model);
CREATE INDEX idx_traces_environment ON traces(environment) WHERE environment IS NOT NULL;
CREATE INDEX idx_traces_tags ON traces USING GIN(tags);
CREATE INDEX idx_traces_request_data ON traces USING GIN(request_data);
CREATE INDEX idx_traces_response_data ON traces USING GIN(response_data);

-- Test Runs table
CREATE TABLE test_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    run_id VARCHAR(255) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    git_sha VARCHAR(40) NOT NULL,
    git_branch VARCHAR(255),
    git_commit_message TEXT,
    ci_provider VARCHAR(50),
    ci_pr_number INTEGER,
    total_cases INTEGER NOT NULL DEFAULT 0,
    passed_cases INTEGER NOT NULL DEFAULT 0,
    warned_cases INTEGER NOT NULL DEFAULT 0,
    failed_cases INTEGER NOT NULL DEFAULT 0,
    results JSONB NOT NULL DEFAULT '[]',
    violations JSONB NOT NULL DEFAULT '[]',
    status VARCHAR(50) NOT NULL DEFAULT 'completed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(project_id, run_id),
    CHECK (status IN ('running', 'completed', 'failed', 'cancelled'))
);

CREATE INDEX idx_test_runs_project_id ON test_runs(project_id);
CREATE INDEX idx_test_runs_timestamp ON test_runs(timestamp DESC);
CREATE INDEX idx_test_runs_git_sha ON test_runs(git_sha);
CREATE INDEX idx_test_runs_git_branch ON test_runs(git_branch);
CREATE INDEX idx_test_runs_ci_pr_number ON test_runs(ci_pr_number) WHERE ci_pr_number IS NOT NULL;
CREATE INDEX idx_test_runs_status ON test_runs(status);
CREATE INDEX idx_test_runs_results ON test_runs USING GIN(results);

-- Regression Detections table
CREATE TABLE regression_detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    case_id VARCHAR(255) NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    regression_git_sha VARCHAR(40) NOT NULL,
    last_good_git_sha VARCHAR(40),
    regression_type VARCHAR(50) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    details JSONB NOT NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_regression_detections_project_id ON regression_detections(project_id);
CREATE INDEX idx_regression_detections_case_id ON regression_detections(case_id);
CREATE INDEX idx_regression_detections_regression_git_sha ON regression_detections(regression_git_sha);
CREATE INDEX idx_regression_detections_detected_at ON regression_detections(detected_at DESC);
CREATE INDEX idx_regression_detections_resolved ON regression_detections(resolved_at) WHERE resolved_at IS NULL;

-- Usage Records table
CREATE TABLE usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_count INTEGER NOT NULL DEFAULT 1,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    billing_period VARCHAR(7) NOT NULL,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_usage_organization_id ON usage_records(organization_id);
CREATE INDEX idx_usage_billing_period ON usage_records(billing_period);
CREATE INDEX idx_usage_resource_type ON usage_records(resource_type);
CREATE INDEX idx_usage_recorded_at ON usage_records(recorded_at DESC);

-- GitHub Installations table
CREATE TABLE github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    installation_id BIGINT UNIQUE NOT NULL,
    account_type VARCHAR(50) NOT NULL,
    account_login VARCHAR(255) NOT NULL,
    repositories JSONB,
    permissions JSONB,
    suspended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (account_type IN ('Organization', 'User'))
);

CREATE INDEX idx_github_installations_organization_id ON github_installations(organization_id);
CREATE INDEX idx_github_installations_installation_id ON github_installations(installation_id);
