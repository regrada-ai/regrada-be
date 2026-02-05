-- Update user_role enum: rename 'user' to 'member' and 'readonly-user' to 'viewer'

-- First rename the enum values (this automatically updates all existing data)
ALTER TYPE user_role RENAME VALUE 'user' TO 'member';
ALTER TYPE user_role RENAME VALUE 'readonly-user' TO 'viewer';

-- Update defaults
ALTER TABLE organization_members ALTER COLUMN role SET DEFAULT 'member';
ALTER TABLE invites ALTER COLUMN role SET DEFAULT 'member';

-- Update check constraint on users table (users.role is VARCHAR, not enum, so we need to update values)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'member' WHERE role = 'user';
UPDATE users SET role = 'viewer' WHERE role = 'readonly-user';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'member', 'viewer'));
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'member';
