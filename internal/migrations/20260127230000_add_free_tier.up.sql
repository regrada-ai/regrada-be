-- Add 'free' tier to organizations table
ALTER TABLE organizations
DROP CONSTRAINT IF EXISTS organizations_tier_check;

ALTER TABLE organizations
ADD CONSTRAINT organizations_tier_check
CHECK (tier IN ('free', 'standard', 'pro', 'enterprise'));

-- Add 'free' tier to api_keys table
ALTER TABLE api_keys
DROP CONSTRAINT IF EXISTS api_keys_tier_check;

ALTER TABLE api_keys
ADD CONSTRAINT api_keys_tier_check
CHECK (tier IN ('free', 'standard', 'pro', 'enterprise'));

-- Update default tier for new organizations to 'free'
ALTER TABLE organizations
ALTER COLUMN tier SET DEFAULT 'free';
