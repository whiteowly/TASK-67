-- +goose Up
CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    actor_type  VARCHAR(32)  NOT NULL DEFAULT 'user', -- 'user', 'system', 'device'
    actor_id    UUID,
    action      VARCHAR(64)  NOT NULL,
    resource    VARCHAR(64)  NOT NULL,
    resource_id TEXT,
    old_state   JSONB,
    new_state   JSONB,
    reason_code VARCHAR(64),
    note        TEXT,
    request_id  UUID,
    ip_addr     INET,
    metadata    JSONB,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- No UPDATE or DELETE should ever be issued on this table from the application.
-- Indexes for common query patterns
CREATE INDEX idx_audit_logs_actor ON audit_logs (actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource, resource_id, created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs (action, created_at DESC);
CREATE INDEX idx_audit_logs_created ON audit_logs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
