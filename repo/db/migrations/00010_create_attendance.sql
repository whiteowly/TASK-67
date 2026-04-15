-- +goose Up

-- Check-in events (immutable)
CREATE TABLE check_in_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id UUID         NOT NULL REFERENCES session_registrations(id),
    session_id      UUID         NOT NULL REFERENCES program_sessions(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    method          VARCHAR(32)  NOT NULL DEFAULT 'qr_staff', -- qr_staff, beacon, manual
    confirmed_by    UUID         REFERENCES users(id), -- staff who confirmed
    device_id       UUID,
    valid           BOOLEAN      NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_checkin_registration ON check_in_events (registration_id, created_at DESC);

-- Occupancy sessions (active seating period)
CREATE TABLE occupancy_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id UUID         NOT NULL REFERENCES session_registrations(id),
    session_id      UUID         NOT NULL REFERENCES program_sessions(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ,
    end_reason      VARCHAR(32), -- completed, leave_breach, timeout, admin_release
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_occupancy_active ON occupancy_sessions (registration_id)
    WHERE is_active = true;

-- Temporary leave events
CREATE TABLE temporary_leave_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occupancy_id    UUID         NOT NULL REFERENCES occupancy_sessions(id),
    registration_id UUID         NOT NULL REFERENCES session_registrations(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    left_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    returned_at     TIMESTAMPTZ,
    max_duration_minutes INTEGER NOT NULL DEFAULT 10,
    exceeded        BOOLEAN      NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_leave_occupancy ON temporary_leave_events (occupancy_id);

-- Occupancy exceptions
CREATE TABLE occupancy_exceptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id UUID         NOT NULL REFERENCES session_registrations(id),
    session_id      UUID         NOT NULL REFERENCES program_sessions(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    exception_type  VARCHAR(64)  NOT NULL, -- leave_breach, unverified_occupancy, manual
    description     TEXT,
    ticket_id       UUID, -- linked to tickets table (created in phase 4)
    resolved        BOOLEAN      NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_occ_exception_session ON occupancy_exceptions (session_id, created_at DESC);

-- Kiosk scan events
CREATE TABLE kiosk_scan_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id       UUID,
    registration_id UUID         REFERENCES session_registrations(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    session_id      UUID         NOT NULL REFERENCES program_sessions(id),
    confirmed_by    UUID         REFERENCES users(id),
    scan_data       TEXT,
    result          VARCHAR(32)  NOT NULL DEFAULT 'pending', -- pending, confirmed, rejected
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Beacon verifications
CREATE TABLE beacon_verifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id UUID         NOT NULL REFERENCES session_registrations(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    beacon_id       VARCHAR(128),
    signal_strength INTEGER,
    verified_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS beacon_verifications;
DROP TABLE IF EXISTS kiosk_scan_events;
DROP TABLE IF EXISTS occupancy_exceptions;
DROP TABLE IF EXISTS temporary_leave_events;
DROP TABLE IF EXISTS occupancy_sessions;
DROP TABLE IF EXISTS check_in_events;
