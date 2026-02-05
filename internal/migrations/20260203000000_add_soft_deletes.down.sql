-- Remove soft delete columns

-- Remove deleted_at from traces table
DROP INDEX IF EXISTS idx_traces_deleted_at;
ALTER TABLE traces DROP COLUMN IF EXISTS deleted_at;

-- Remove deleted_at from test_runs table
DROP INDEX IF EXISTS idx_test_runs_deleted_at;
ALTER TABLE test_runs DROP COLUMN IF EXISTS deleted_at;

-- Remove deleted_at from organization_members table
DROP INDEX IF EXISTS idx_organization_members_deleted_at;
ALTER TABLE organization_members DROP COLUMN IF EXISTS deleted_at;
