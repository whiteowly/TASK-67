-- +goose Up

-- File artifacts
CREATE TABLE file_artifacts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename    VARCHAR(256) NOT NULL,
    file_type   VARCHAR(16)  NOT NULL, -- csv, xlsx
    mime_type   VARCHAR(128),
    size_bytes  BIGINT       NOT NULL DEFAULT 0,
    checksum    VARCHAR(128) NOT NULL, -- SHA-256
    storage_path TEXT        NOT NULL,
    artifact_type VARCHAR(32) NOT NULL, -- import, export
    uploaded_by UUID         REFERENCES users(id),
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_file_artifacts_checksum ON file_artifacts (checksum, artifact_type);

-- Import jobs
CREATE TABLE import_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_artifact_id UUID       NOT NULL REFERENCES file_artifacts(id),
    template_type   VARCHAR(64) NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'uploaded',
    -- status: uploaded, validating, validation_failed, ready_to_apply, applying,
    --         completed, partially_applied, failed, rejected_duplicate, retry_pending, canceled
    uploaded_by     UUID        NOT NULL REFERENCES users(id),
    total_rows      INTEGER,
    valid_rows      INTEGER,
    error_rows      INTEGER,
    applied_rows    INTEGER,
    force_reprocess BOOLEAN     NOT NULL DEFAULT false,
    force_reason    TEXT,
    error_summary   JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_import_jobs_status ON import_jobs (status, created_at DESC);

-- Import rows (row-level validation)
CREATE TABLE import_rows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    import_job_id   UUID        NOT NULL REFERENCES import_jobs(id),
    row_number      INTEGER     NOT NULL,
    raw_data        JSONB       NOT NULL,
    is_valid        BOOLEAN     NOT NULL DEFAULT false,
    errors          JSONB,
    applied         BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_import_rows_job ON import_rows (import_job_id, row_number);

-- Export jobs
CREATE TABLE export_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    export_type     VARCHAR(64) NOT NULL,
    format          VARCHAR(16) NOT NULL DEFAULT 'csv', -- csv, xlsx
    status          VARCHAR(32) NOT NULL DEFAULT 'queued',
    -- status: queued, running, completed, failed, retry_pending, canceled
    filters         JSONB       NOT NULL DEFAULT '{}',
    file_artifact_id UUID       REFERENCES file_artifacts(id),
    requested_by    UUID        NOT NULL REFERENCES users(id),
    total_rows      INTEGER,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_export_jobs_status ON export_jobs (status, created_at DESC);
CREATE INDEX idx_export_jobs_user ON export_jobs (requested_by, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS export_jobs;
DROP TABLE IF EXISTS import_rows;
DROP TABLE IF EXISTS import_jobs;
DROP TABLE IF EXISTS file_artifacts;
