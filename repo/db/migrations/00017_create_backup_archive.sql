-- +goose Up

-- Backup runs
CREATE TABLE backup_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status          VARCHAR(32) NOT NULL DEFAULT 'running',
    -- status: running, completed, failed
    artifact_path   TEXT,
    checksum        VARCHAR(128),
    encrypted       BOOLEAN     NOT NULL DEFAULT true,
    size_bytes      BIGINT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    error           TEXT,
    retention_days  INTEGER     NOT NULL DEFAULT 30,
    expires_at      TIMESTAMPTZ,
    triggered_by    UUID        REFERENCES users(id), -- NULL = scheduled
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_backup_runs_status ON backup_runs (status, created_at DESC);

-- Restore runs
CREATE TABLE restore_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    backup_run_id   UUID        NOT NULL REFERENCES backup_runs(id),
    status          VARCHAR(32) NOT NULL DEFAULT 'pending',
    -- status: pending, dry_run, dry_run_completed, restoring, completed, failed
    is_dry_run      BOOLEAN     NOT NULL DEFAULT true,
    reason          TEXT        NOT NULL,
    initiated_by    UUID        NOT NULL REFERENCES users(id),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    error           TEXT,
    validation_result JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_restore_runs ON restore_runs (created_at DESC);

-- Archive schema
CREATE SCHEMA IF NOT EXISTS archive;

-- Archive runs
CREATE TABLE archive_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    archive_type    VARCHAR(64) NOT NULL, -- orders, tickets
    status          VARCHAR(32) NOT NULL DEFAULT 'running',
    -- status: running, completed, partially_completed, failed
    threshold_date  TIMESTAMPTZ NOT NULL,
    total_rows      INTEGER     NOT NULL DEFAULT 0,
    archived_rows   INTEGER     NOT NULL DEFAULT 0,
    last_cursor     TEXT,
    chunk_size      INTEGER     NOT NULL DEFAULT 1000,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_archive_runs ON archive_runs (archive_type, created_at DESC);

-- Archive lookup projection (masked retained data)
CREATE TABLE archive_lookup_projection (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type     VARCHAR(64) NOT NULL, -- order, ticket
    source_id       UUID        NOT NULL,
    month           DATE        NOT NULL,
    status          VARCHAR(32),
    facility        VARCHAR(128),
    masked_user_ref VARCHAR(64),
    monetary_total  BIGINT,
    currency        CHAR(3)     DEFAULT 'CNY',
    archived_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_archive_lookup ON archive_lookup_projection (source_type, month);
CREATE UNIQUE INDEX idx_archive_lookup_source ON archive_lookup_projection (source_type, source_id);

-- Archive tables (mirroring live tables in archive schema)
CREATE TABLE archive.orders (LIKE orders INCLUDING ALL);
CREATE TABLE archive.order_items (LIKE order_items INCLUDING ALL);
CREATE TABLE archive.tickets (LIKE tickets INCLUDING ALL);
CREATE TABLE archive.ticket_comments (LIKE ticket_comments INCLUDING ALL);

-- +goose Down
DROP TABLE IF EXISTS archive.ticket_comments;
DROP TABLE IF EXISTS archive.tickets;
DROP TABLE IF EXISTS archive.order_items;
DROP TABLE IF EXISTS archive.orders;
DROP TABLE IF EXISTS archive_lookup_projection;
DROP TABLE IF EXISTS archive_runs;
DROP TABLE IF EXISTS restore_runs;
DROP TABLE IF EXISTS backup_runs;
DROP SCHEMA IF EXISTS archive;
