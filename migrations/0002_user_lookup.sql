CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower ON users (lower(username));

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_users_username_trgm ON users USING gin (username gin_trgm_ops);

DROP INDEX IF EXISTS idx_users_username;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
