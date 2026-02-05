-- Revert user_role enum: rename 'member' back to 'user' and 'viewer' back to 'readonly-user'

-- First rename the enum values back (this automatically updates all existing data)
ALTER TYPE user_role RENAME VALUE 'member' TO 'user';
ALTER TYPE user_role RENAME VALUE 'viewer' TO 'readonly-user';

-- Update defaults back
ALTER TABLE organization_members ALTER COLUMN role SET DEFAULT 'user';
ALTER TABLE invites ALTER COLUMN role SET DEFAULT 'user';

-- Revert check constraint on users table (users.role is VARCHAR, not enum, so we need to update values)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'user' WHERE role = 'member';
UPDATE users SET role = 'readonly-user' WHERE role = 'viewer';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'user', 'readonly-user'));
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'user';
