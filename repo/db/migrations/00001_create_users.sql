-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(64)  NOT NULL,
    display_name    VARCHAR(128) NOT NULL DEFAULT '',
    email           VARCHAR(255),
    phone           VARCHAR(32),
    password_hash   TEXT         NOT NULL,
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    failed_attempts INTEGER      NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ,
    last_login_at   TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Case-insensitive unique username for active (non-deleted) users
CREATE UNIQUE INDEX idx_users_username_active ON users (lower(username)) WHERE deleted_at IS NULL;

-- Case-insensitive unique email for active users (nullable)
CREATE UNIQUE INDEX idx_users_email_active ON users (lower(email)) WHERE deleted_at IS NULL AND email IS NOT NULL;

CREATE INDEX idx_users_is_active ON users (is_active) WHERE deleted_at IS NULL;

-- Password history for reuse prevention
CREATE TABLE password_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    hash        TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_history_user ON password_history (user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS password_history;
DROP TABLE IF EXISTS users;
