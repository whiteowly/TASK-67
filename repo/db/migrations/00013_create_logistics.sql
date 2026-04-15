-- +goose Up

-- Shipments
CREATE TABLE shipments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID         NOT NULL REFERENCES orders(id),
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending_fulfillment',
    -- status: pending_fulfillment, packed, shipped, delivered, delivery_exception,
    --         returned, closed_exception, canceled
    tracking_number VARCHAR(128),
    carrier         VARCHAR(64),
    shipped_by      UUID         REFERENCES users(id),
    shipped_at      TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_shipments_order ON shipments (order_id);
CREATE INDEX idx_shipments_status ON shipments (status);

-- Shipment status history (immutable)
CREATE TABLE shipment_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipment_id UUID         NOT NULL REFERENCES shipments(id),
    old_status  VARCHAR(32),
    new_status  VARCHAR(32)  NOT NULL,
    actor_id    UUID,
    note        TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_shipment_history ON shipment_status_history (shipment_id, created_at);

-- Delivery proofs
CREATE TABLE delivery_proofs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipment_id     UUID         NOT NULL REFERENCES shipments(id),
    proof_type      VARCHAR(32)  NOT NULL, -- 'signature_image', 'typed_acknowledgment'
    signature_data  BYTEA,
    acknowledgment_text TEXT,
    receiver_name   VARCHAR(128),
    delivered_at    TIMESTAMPTZ  NOT NULL,
    recorded_by     UUID         NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_delivery_proof_shipment ON delivery_proofs (shipment_id);

-- Delivery exceptions
CREATE TABLE delivery_exceptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipment_id     UUID         NOT NULL REFERENCES shipments(id),
    exception_type  VARCHAR(64)  NOT NULL,
    description     TEXT         NOT NULL,
    reported_by     UUID         NOT NULL REFERENCES users(id),
    resolved        BOOLEAN      NOT NULL DEFAULT false,
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_delivery_exception_shipment ON delivery_exceptions (shipment_id);

-- +goose Down
DROP TABLE IF EXISTS delivery_exceptions;
DROP TABLE IF EXISTS delivery_proofs;
DROP TABLE IF EXISTS shipment_status_history;
DROP TABLE IF EXISTS shipments;
