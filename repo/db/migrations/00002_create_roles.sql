-- +goose Up
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(64)  NOT NULL UNIQUE,
    description TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE user_role_assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id),
    role_id         UUID        NOT NULL REFERENCES roles(id),
    assigned_by     UUID        REFERENCES users(id),
    effective_from  TIMESTAMPTZ NOT NULL DEFAULT now(),
    effective_until  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_user_role_active ON user_role_assignments (user_id, role_id)
    WHERE effective_until IS NULL;

CREATE INDEX idx_user_role_user ON user_role_assignments (user_id) WHERE effective_until IS NULL;

-- Seed the four required roles
INSERT INTO roles (name, description) VALUES
    ('member', 'End user: register for sessions, purchase products, view own data'),
    ('staff', 'Operational user: attendance, shipping, delivery, exceptions'),
    ('moderator', 'Content moderation: reviews, reports, bans'),
    ('administrator', 'Full administrative access: policies, exports, archival, overrides');

-- +goose Down
DROP TABLE IF EXISTS user_role_assignments;
DROP TABLE IF EXISTS roles;
