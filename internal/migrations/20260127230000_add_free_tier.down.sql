-- Revert default tier for organizations back to 'standard'
ALTER TABLE organizations
ALTER COLUMN tier SET DEFAULT 'standard';

-- Remove 'free' tier from api_keys table
ALTER TABLE api_keys
DROP CONSTRAINT IF EXISTS api_keys_tier_check;

ALTER TABLE api_keys
ADD CONSTRAINT api_keys_tier_check
CHECK (tier IN ('standard', 'pro', 'enterprise'));

-- Remove 'free' tier from organizations table
ALTER TABLE organizations
DROP CONSTRAINT IF EXISTS organizations_tier_check;

ALTER TABLE organizations
ADD CONSTRAINT organizations_tier_check
CHECK (tier IN ('standard', 'pro', 'enterprise'));
