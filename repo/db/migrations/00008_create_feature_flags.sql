-- +goose Up
CREATE TABLE feature_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key             VARCHAR(128) NOT NULL UNIQUE,
    enabled         BOOLEAN      NOT NULL DEFAULT false,
    description     TEXT         NOT NULL DEFAULT '',
    cohort_percent  INTEGER      NOT NULL DEFAULT 100 CHECK (cohort_percent BETWEEN 0 AND 100),
    target_roles    TEXT[]       DEFAULT '{}',
    target_domains  TEXT[]       DEFAULT '{}',
    metadata        JSONB        NOT NULL DEFAULT '{}',
    updated_by      UUID         REFERENCES users(id),
    version         INTEGER      NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS feature_flags;
