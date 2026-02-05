-- Change unique constraint to only apply to pending invites (not revoked, not accepted)

-- Drop the existing unique constraint
ALTER TABLE invites DROP CONSTRAINT IF EXISTS invites_organization_id_email_key;

-- Create a partial unique index that only applies to pending invites
CREATE UNIQUE INDEX invites_organization_id_email_pending_key
ON invites (organization_id, email)
WHERE accepted_at IS NULL AND revoked_at IS NULL;
