-- +goose Up

-- Tickets (exception/claims)
CREATE TABLE tickets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_number   VARCHAR(32)  NOT NULL UNIQUE,
    ticket_type     VARCHAR(64)  NOT NULL,
    -- types: occupancy_exception, delivery_exception, late_payment_reconciliation,
    --        refund_dispute, moderation_escalation, import_failure, check_in_dispute
    title           VARCHAR(256) NOT NULL,
    description     TEXT,
    priority        VARCHAR(16)  NOT NULL DEFAULT 'medium',
    -- priority: low, medium, high, critical
    status          VARCHAR(32)  NOT NULL DEFAULT 'open',
    -- status: open, acknowledged, in_progress, waiting_on_member, waiting_on_staff,
    --         escalated, resolved, reopened, closed
    source_type     VARCHAR(64),
    source_id       UUID,
    assigned_to     UUID         REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    resolution_code VARCHAR(64),
    resolution_summary TEXT,
    closed_at       TIMESTAMPTZ,
    closed_by       UUID         REFERENCES users(id),
    sla_response_due TIMESTAMPTZ,
    sla_resolution_due TIMESTAMPTZ,
    sla_response_met BOOLEAN,
    sla_resolution_met BOOLEAN,
    created_by      UUID         REFERENCES users(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_tickets_status ON tickets (status, priority DESC);
CREATE INDEX idx_tickets_assigned ON tickets (assigned_to, status);
CREATE INDEX idx_tickets_type ON tickets (ticket_type, status);
CREATE INDEX idx_tickets_sla ON tickets (sla_response_due) WHERE sla_response_met IS NULL;

-- Ticket assignments (history)
CREATE TABLE ticket_assignments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID         NOT NULL REFERENCES tickets(id),
    assigned_to UUID         NOT NULL REFERENCES users(id),
    assigned_by UUID         REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_ticket_assign ON ticket_assignments (ticket_id, created_at);

-- Ticket comments
CREATE TABLE ticket_comments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID         NOT NULL REFERENCES tickets(id),
    author_id   UUID         NOT NULL REFERENCES users(id),
    body        TEXT         NOT NULL,
    is_internal BOOLEAN      NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_ticket_comments ON ticket_comments (ticket_id, created_at);

-- Ticket status history
CREATE TABLE ticket_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID         NOT NULL REFERENCES tickets(id),
    old_status  VARCHAR(32),
    new_status  VARCHAR(32)  NOT NULL,
    actor_id    UUID,
    reason      TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_ticket_status_hist ON ticket_status_history (ticket_id, created_at);

-- Ticket SLA events
CREATE TABLE ticket_sla_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID         NOT NULL REFERENCES tickets(id),
    event_type  VARCHAR(64)  NOT NULL, -- response_due, resolution_due, response_breach, resolution_breach, reminder
    due_at      TIMESTAMPTZ,
    fired_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_ticket_sla_events ON ticket_sla_events (ticket_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS ticket_sla_events;
DROP TABLE IF EXISTS ticket_status_history;
DROP TABLE IF EXISTS ticket_comments;
DROP TABLE IF EXISTS ticket_assignments;
DROP TABLE IF EXISTS tickets;
