-- Migration: 001_create_users

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE SEQUENCE IF NOT EXISTS user_id_seq START 100000000000;

-- Up
CREATE TABLE IF NOT EXISTS users (
    id          BIGINT      PRIMARY KEY DEFAULT nextval('user_id_seq'),
    username    VARCHAR(24) NOT NULL UNIQUE,
    password    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);

-- Auto-update updated_at on row change
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Down (uncomment to roll back)
-- DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
-- DROP FUNCTION IF EXISTS set_updated_at();
-- DROP TABLE IF EXISTS users;