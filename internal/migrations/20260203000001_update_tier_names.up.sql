-- Update tier names: free->starter, standard->team, pro->scale
-- Also add monthly usage limit columns

-- First, drop the existing constraints
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_tier_check;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_tier_check;

-- Update existing tier values in organizations
UPDATE organizations SET tier = 'starter' WHERE tier = 'free';
UPDATE organizations SET tier = 'team' WHERE tier = 'standard';
UPDATE organizations SET tier = 'scale' WHERE tier = 'pro';

-- Update existing tier values in api_keys
UPDATE api_keys SET tier = 'starter' WHERE tier = 'free';
UPDATE api_keys SET tier = 'team' WHERE tier = 'standard';
UPDATE api_keys SET tier = 'scale' WHERE tier = 'pro';

-- Add new constraints with updated tier names
ALTER TABLE organizations
ADD CONSTRAINT organizations_tier_check
CHECK (tier IN ('starter', 'team', 'scale', 'enterprise'));

ALTER TABLE api_keys
ADD CONSTRAINT api_keys_tier_check
CHECK (tier IN ('starter', 'team', 'scale', 'enterprise'));

-- Add monthly usage limit column to organizations
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS monthly_request_limit BIGINT NOT NULL DEFAULT 50000;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS monthly_request_count BIGINT NOT NULL DEFAULT 0;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS usage_reset_at TIMESTAMPTZ NOT NULL DEFAULT date_trunc('month', now() AT TIME ZONE 'UTC') + INTERVAL '1 month';

-- Set default monthly limits based on tier
UPDATE organizations SET monthly_request_limit = 50000 WHERE tier = 'starter';
UPDATE organizations SET monthly_request_limit = 500000 WHERE tier = 'team';
UPDATE organizations SET monthly_request_limit = 5000000 WHERE tier = 'scale';
UPDATE organizations SET monthly_request_limit = 20000000 WHERE tier = 'enterprise';

-- Update rate limits in api_keys to match new values
UPDATE api_keys SET rate_limit_rpm = 10 WHERE tier = 'starter';
UPDATE api_keys SET rate_limit_rpm = 100 WHERE tier = 'team';
UPDATE api_keys SET rate_limit_rpm = 500 WHERE tier = 'scale';
UPDATE api_keys SET rate_limit_rpm = 2000 WHERE tier = 'enterprise';
