-- Remove single organization per user constraint

DROP INDEX IF EXISTS organization_members_user_id_active_key;
