-- +goose Up

-- Session registration records
CREATE TABLE session_registrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID         NOT NULL REFERENCES program_sessions(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending_approval',
    -- status: pending_approval, registered, waitlisted, checked_in, temporarily_away,
    --         completed, canceled, rejected, no_show_canceled, released, expired
    registered_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    canceled_at     TIMESTAMPTZ,
    cancel_reason   VARCHAR(64),
    approved_by     UUID         REFERENCES users(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- One active registration per member/session
CREATE UNIQUE INDEX idx_registration_active ON session_registrations (user_id, session_id)
    WHERE status IN ('pending_approval', 'registered', 'waitlisted', 'checked_in', 'temporarily_away');

CREATE INDEX idx_registration_session ON session_registrations (session_id, status);
CREATE INDEX idx_registration_user ON session_registrations (user_id, created_at DESC);

-- Registration status history (immutable)
CREATE TABLE registration_status_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id UUID         NOT NULL REFERENCES session_registrations(id),
    old_status      VARCHAR(32),
    new_status      VARCHAR(32)  NOT NULL,
    actor_type      VARCHAR(32)  NOT NULL DEFAULT 'user', -- user, system, admin
    actor_id        UUID,
    reason_code     VARCHAR(64),
    note            TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_reg_history_registration ON registration_status_history (registration_id, created_at);

-- Waitlist entries (ordered)
CREATE TABLE session_waitlist_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID         NOT NULL REFERENCES program_sessions(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    registration_id UUID         NOT NULL REFERENCES session_registrations(id),
    position        INTEGER      NOT NULL,
    status          VARCHAR(32)  NOT NULL DEFAULT 'waiting',
    -- status: waiting, promoted, expired, canceled
    promoted_at     TIMESTAMPTZ,
    promotion_attempts INTEGER   NOT NULL DEFAULT 0,
    last_attempt_reason TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_waitlist_active ON session_waitlist_entries (user_id, session_id)
    WHERE status = 'waiting';

CREATE INDEX idx_waitlist_session_pos ON session_waitlist_entries (session_id, position)
    WHERE status = 'waiting';

-- Session policies (versioned attendance/seat rules)
CREATE TABLE session_policies (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id              UUID REFERENCES program_sessions(id), -- NULL = facility default
    checkin_lead_minutes    INTEGER NOT NULL DEFAULT 30,
    noshow_cancel_minutes   INTEGER NOT NULL DEFAULT 10,
    leave_max_minutes       INTEGER NOT NULL DEFAULT 10,
    leave_per_hour          INTEGER NOT NULL DEFAULT 1,
    unverified_threshold_minutes INTEGER NOT NULL DEFAULT 15,
    requires_beacon         BOOLEAN NOT NULL DEFAULT false,
    version                 INTEGER NOT NULL DEFAULT 1,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS session_policies;
DROP TABLE IF EXISTS session_waitlist_entries;
DROP TABLE IF EXISTS registration_status_history;
DROP TABLE IF EXISTS session_registrations;
