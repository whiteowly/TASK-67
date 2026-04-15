-- +goose Up

-- Posts (user-generated content)
CREATE TABLE posts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    title       VARCHAR(256),
    body        TEXT         NOT NULL CHECK (length(body) <= 5000),
    status      VARCHAR(32)  NOT NULL DEFAULT 'active',
    -- status: active, hidden, removed
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_posts_user ON posts (user_id, created_at DESC);
CREATE INDEX idx_posts_status ON posts (status, created_at DESC) WHERE deleted_at IS NULL;

-- Post reports (abuse)
CREATE TABLE post_reports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID         NOT NULL REFERENCES posts(id),
    reporter_id UUID         NOT NULL REFERENCES users(id),
    reason      VARCHAR(64)  NOT NULL,
    description TEXT,
    status      VARCHAR(32)  NOT NULL DEFAULT 'open',
    -- status: open, reviewed, dismissed, actioned
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Dedup: same reporter cannot create duplicate open reports for same post+reason within 24h
CREATE UNIQUE INDEX idx_report_dedup ON post_reports (reporter_id, post_id, reason)
    WHERE status = 'open';

CREATE INDEX idx_reports_post ON post_reports (post_id);
CREATE INDEX idx_reports_status ON post_reports (status, created_at DESC);

-- Moderation cases
CREATE TABLE moderation_cases (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID         REFERENCES posts(id),
    user_id     UUID         REFERENCES users(id), -- subject user
    status      VARCHAR(32)  NOT NULL DEFAULT 'open',
    -- status: open, under_review, escalated, actioned, dismissed
    assigned_to UUID         REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_mod_cases_status ON moderation_cases (status, created_at DESC);

-- Moderation actions (immutable)
CREATE TABLE moderation_actions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id     UUID         NOT NULL REFERENCES moderation_cases(id),
    action_type VARCHAR(64)  NOT NULL, -- warn, hide_post, remove_post, ban_user, dismiss
    actor_id    UUID         NOT NULL REFERENCES users(id),
    details     JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_mod_actions_case ON moderation_actions (case_id, created_at);

-- Account bans
CREATE TABLE account_bans (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    ban_type    VARCHAR(32)  NOT NULL DEFAULT 'posting',
    -- ban_type: posting, platform_wide
    is_permanent BOOLEAN     NOT NULL DEFAULT false,
    starts_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    ends_at     TIMESTAMPTZ,
    reason      TEXT         NOT NULL,
    issued_by   UUID         NOT NULL REFERENCES users(id),
    revoked_at  TIMESTAMPTZ,
    revoked_by  UUID         REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_bans_user ON account_bans (user_id, starts_at DESC);
CREATE INDEX idx_bans_user_active ON account_bans (user_id, starts_at DESC)
    WHERE revoked_at IS NULL;

-- Posting rate windows
CREATE TABLE posting_rate_windows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    posted_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_rate_window ON posting_rate_windows (user_id, posted_at DESC);

-- +goose Down
DROP TABLE IF EXISTS posting_rate_windows;
DROP TABLE IF EXISTS account_bans;
DROP TABLE IF EXISTS moderation_actions;
DROP TABLE IF EXISTS moderation_cases;
DROP TABLE IF EXISTS post_reports;
DROP TABLE IF EXISTS posts;
