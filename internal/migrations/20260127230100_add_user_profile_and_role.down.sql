-- Remove index on role
DROP INDEX IF EXISTS idx_users_role;

-- Remove check constraint
ALTER TABLE users
DROP CONSTRAINT IF EXISTS users_role_check;

-- Remove profile_picture and role columns
ALTER TABLE users
DROP COLUMN IF EXISTS profile_picture,
DROP COLUMN IF EXISTS role;
