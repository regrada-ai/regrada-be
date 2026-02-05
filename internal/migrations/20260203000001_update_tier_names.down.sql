-- Revert tier names: starter->free, team->standard, scale->pro
-- Remove monthly usage limit columns

-- First, drop the new constraints
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_tier_check;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_tier_check;

-- Revert tier values in organizations
UPDATE organizations SET tier = 'free' WHERE tier = 'starter';
UPDATE organizations SET tier = 'standard' WHERE tier = 'team';
UPDATE organizations SET tier = 'pro' WHERE tier = 'scale';

-- Revert tier values in api_keys
UPDATE api_keys SET tier = 'free' WHERE tier = 'starter';
UPDATE api_keys SET tier = 'standard' WHERE tier = 'team';
UPDATE api_keys SET tier = 'pro' WHERE tier = 'scale';

-- Add back old constraints
ALTER TABLE organizations
ADD CONSTRAINT organizations_tier_check
CHECK (tier IN ('free', 'standard', 'pro', 'enterprise'));

ALTER TABLE api_keys
ADD CONSTRAINT api_keys_tier_check
CHECK (tier IN ('free', 'standard', 'pro', 'enterprise'));

-- Remove monthly usage columns
ALTER TABLE organizations DROP COLUMN IF EXISTS monthly_request_limit;
ALTER TABLE organizations DROP COLUMN IF EXISTS monthly_request_count;
ALTER TABLE organizations DROP COLUMN IF EXISTS usage_reset_at;

-- Revert rate limits in api_keys
UPDATE api_keys SET rate_limit_rpm = 10 WHERE tier = 'free';
UPDATE api_keys SET rate_limit_rpm = 100 WHERE tier = 'standard';
UPDATE api_keys SET rate_limit_rpm = 500 WHERE tier = 'pro';
UPDATE api_keys SET rate_limit_rpm = 2000 WHERE tier = 'enterprise';
