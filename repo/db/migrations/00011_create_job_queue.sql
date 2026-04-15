-- +goose Up

-- Durable job queue
CREATE TABLE job_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type        VARCHAR(64)  NOT NULL,
    payload         JSONB        NOT NULL DEFAULT '{}',
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending',
    -- status: pending, running, completed, failed, canceled
    priority        INTEGER      NOT NULL DEFAULT 0,
    max_retries     INTEGER      NOT NULL DEFAULT 3,
    retry_count     INTEGER      NOT NULL DEFAULT 0,
    run_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    last_error      TEXT,
    lease_token     UUID,
    lease_expires   TIMESTAMPTZ,
    progress        JSONB,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_queue_pending ON job_queue (run_at, priority DESC)
    WHERE status = 'pending';
CREATE INDEX idx_job_queue_type ON job_queue (job_type, status);
CREATE INDEX idx_job_queue_lease ON job_queue (lease_expires)
    WHERE status = 'running';

-- Job attempt history
CREATE TABLE job_attempts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID         NOT NULL REFERENCES job_queue(id),
    attempt     INTEGER      NOT NULL,
    started_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    status      VARCHAR(32)  NOT NULL DEFAULT 'running',
    error       TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_attempts_job ON job_attempts (job_id, attempt);

-- Scheduled jobs configuration
CREATE TABLE scheduled_jobs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(128) NOT NULL UNIQUE,
    job_type    VARCHAR(64)  NOT NULL,
    cron_expr   VARCHAR(64)  NOT NULL, -- cron expression
    enabled     BOOLEAN      NOT NULL DEFAULT true,
    payload     JSONB        NOT NULL DEFAULT '{}',
    last_run    TIMESTAMPTZ,
    next_run    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Seed scheduled jobs
INSERT INTO scheduled_jobs (name, job_type, cron_expr) VALUES
    ('payment_expiry', 'payment_expiry_closure', '*/1 * * * *'),
    ('noshow_cancel', 'noshow_cancellation', '*/1 * * * *'),
    ('waitlist_promotion', 'waitlist_promotion_scan', '*/1 * * * *'),
    ('stale_occupancy', 'stale_occupancy_detection', '*/5 * * * *'),
    ('sla_reminder', 'sla_reminder_generation', '*/15 * * * *'),
    ('nightly_backup', 'backup_execution', '0 2 * * *'),
    ('nightly_archive', 'archive_execution', '0 3 * * *');

-- +goose Down
DROP TABLE IF EXISTS scheduled_jobs;
DROP TABLE IF EXISTS job_attempts;
DROP TABLE IF EXISTS job_queue;
