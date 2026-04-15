-- +goose Up
CREATE TABLE system_config (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         VARCHAR(128) NOT NULL UNIQUE,
    value       TEXT         NOT NULL,
    value_type  VARCHAR(16)  NOT NULL DEFAULT 'string'
                CHECK (value_type IN ('string', 'int', 'bool', 'json')),
    description TEXT         NOT NULL DEFAULT '',
    updated_by  UUID         REFERENCES users(id),
    version     INTEGER      NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Seed default configuration
INSERT INTO system_config (key, value, value_type, description) VALUES
    ('facility.timezone', 'Asia/Shanghai', 'string', 'Authoritative facility timezone'),
    ('facility.name', 'CampusRec Center', 'string', 'Facility display name'),
    ('registration.default_close_hours_before', '2', 'int', 'Hours before session start to close registration'),
    ('attendance.checkin_lead_minutes', '30', 'int', 'Minutes before session start that check-in opens'),
    ('attendance.noshow_cancel_minutes', '10', 'int', 'Minutes after session start to auto-cancel no-shows'),
    ('attendance.leave_max_minutes', '10', 'int', 'Maximum single temporary leave duration in minutes'),
    ('attendance.unverified_occupancy_threshold_minutes', '15', 'int', 'Minutes before unverified occupancy creates exception'),
    ('payment.request_expiry_minutes', '15', 'int', 'Payment request expiry in minutes'),
    ('moderation.posts_per_hour', '5', 'int', 'Maximum posts per member per rolling hour'),
    ('moderation.abuse_threshold_count', '3', 'int', 'Upheld violations in window to trigger ban review'),
    ('moderation.abuse_threshold_days', '30', 'int', 'Rolling window days for abuse threshold'),
    ('ticket.initial_response_hours', '4', 'int', 'SLA initial response in business hours'),
    ('ticket.resolution_days', '3', 'int', 'SLA resolution in calendar days'),
    ('session.idle_timeout_member_hours', '8', 'int', 'Member session idle timeout hours'),
    ('session.idle_timeout_staff_minutes', '30', 'int', 'Staff/Moderator/Admin session idle timeout minutes'),
    ('archive.threshold_months', '24', 'int', 'Months after close before archiving'),
    ('import.max_file_size_mb', '25', 'int', 'Maximum import file size in MB'),
    ('waitlist.promotion_timeout_seconds', '30', 'int', 'Seconds to complete waitlist promotion');

-- +goose Down
DROP TABLE IF EXISTS system_config;
