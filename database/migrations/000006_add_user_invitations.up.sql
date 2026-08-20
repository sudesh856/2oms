ALTER TABLE users
ADD COLUMN invitation_token_hash TEXT,
ADD COLUMN invitation_expires_at TIMESTAMPTZ;