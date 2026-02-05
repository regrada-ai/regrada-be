-- Restore role column to users table

ALTER TABLE users
ADD COLUMN role VARCHAR(50) NOT NULL DEFAULT 'member';

ALTER TABLE users
ADD CONSTRAINT users_role_check
CHECK (role IN ('admin', 'member', 'viewer'));

CREATE INDEX idx_users_role ON users(role);
