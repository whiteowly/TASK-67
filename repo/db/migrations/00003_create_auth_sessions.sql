-- +goose Up
CREATE TABLE auth_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    token_hash  TEXT         NOT NULL UNIQUE,
    ip_addr     INET,
    user_agent  TEXT,
    expires_at  TIMESTAMPTZ  NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_sessions_user ON auth_sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_auth_sessions_expires ON auth_sessions (expires_at) WHERE revoked_at IS NULL;

-- Account lockout tracking (separate table for clearer history)
CREATE TABLE account_lockouts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    reason      VARCHAR(64)  NOT NULL DEFAULT 'failed_login',
    locked_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    unlocks_at  TIMESTAMPTZ  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_account_lockouts_user ON account_lockouts (user_id, unlocks_at DESC);

-- Device clients for kiosk/staff device registrations
CREATE TABLE device_clients (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(128) NOT NULL,
    device_type     VARCHAR(32)  NOT NULL, -- 'kiosk', 'staff_terminal'
    api_key_hash    TEXT         NOT NULL UNIQUE,
    permissions     JSONB        NOT NULL DEFAULT '[]',
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    registered_by   UUID         REFERENCES users(id),
    last_seen_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS device_clients;
DROP TABLE IF EXISTS account_lockouts;
DROP TABLE IF EXISTS auth_sessions;
