DROP TRIGGER IF EXISTS update_invites_updated_at ON invites;
DROP TRIGGER IF EXISTS update_organization_members_updated_at ON organization_members;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_role;
