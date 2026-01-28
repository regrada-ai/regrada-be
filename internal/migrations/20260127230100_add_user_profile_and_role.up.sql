-- Add profile_picture and role columns to users table
ALTER TABLE users
ADD COLUMN profile_picture TEXT,
ADD COLUMN role VARCHAR(50) NOT NULL DEFAULT 'user';

-- Add check constraint for role
ALTER TABLE users
ADD CONSTRAINT users_role_check
CHECK (role IN ('admin', 'user', 'readonly-user'));

-- Create index on role for faster queries
CREATE INDEX idx_users_role ON users(role);
