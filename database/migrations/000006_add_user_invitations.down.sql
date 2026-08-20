ALTER TABLE users
DROP COLUMN IF EXISTS invitation_token_hash,
DROP COLUMN IF EXISTS invitation_expires_at;