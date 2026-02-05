-- Add soft delete columns to tables that don't have them

-- Add deleted_at to traces table
ALTER TABLE traces ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_traces_deleted_at ON traces(deleted_at) WHERE deleted_at IS NULL;

-- Add deleted_at to test_runs table
ALTER TABLE test_runs ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_test_runs_deleted_at ON test_runs(deleted_at) WHERE deleted_at IS NULL;

-- Add deleted_at to organization_members table
ALTER TABLE organization_members ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_organization_members_deleted_at ON organization_members(deleted_at) WHERE deleted_at IS NULL;
