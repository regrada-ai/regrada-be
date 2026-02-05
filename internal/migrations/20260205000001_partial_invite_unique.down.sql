-- Revert to full unique constraint on organization_id, email

-- Drop the partial unique index
DROP INDEX IF EXISTS invites_organization_id_email_pending_key;

-- Restore the original unique constraint
ALTER TABLE invites ADD CONSTRAINT invites_organization_id_email_key UNIQUE (organization_id, email);
