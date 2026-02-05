-- Enforce that each user can only belong to one organization
-- Uses partial index to respect soft deletes

CREATE UNIQUE INDEX organization_members_user_id_active_key
ON organization_members (user_id)
WHERE deleted_at IS NULL;
