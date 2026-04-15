-- +goose Up

-- Carts
CREATE TABLE carts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    is_active   BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_cart_active ON carts (user_id) WHERE is_active = true;

-- Cart items
CREATE TABLE cart_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id         UUID         NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    item_type       VARCHAR(32)  NOT NULL, -- 'session', 'product'
    item_id         UUID         NOT NULL,
    quantity        INTEGER      NOT NULL DEFAULT 1 CHECK (quantity > 0),
    price_snapshot  BIGINT       NOT NULL CHECK (price_snapshot >= 0),
    currency        CHAR(3)      NOT NULL DEFAULT 'CNY',
    metadata        JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_cart_item_dedup ON cart_items (cart_id, item_type, item_id);

-- Orders
CREATE TABLE orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID         NOT NULL REFERENCES users(id),
    order_number        VARCHAR(32)  NOT NULL UNIQUE,
    status              VARCHAR(32)  NOT NULL DEFAULT 'draft',
    -- status: draft, awaiting_payment, paid, fulfillment_pending, shipped, delivered,
    --         auto_closed, manually_canceled, refund_pending, refunded_partial, refunded_full,
    --         delivery_exception, closed_exception
    subtotal            BIGINT       NOT NULL DEFAULT 0,
    total               BIGINT       NOT NULL DEFAULT 0,
    currency            CHAR(3)      NOT NULL DEFAULT 'CNY',
    delivery_address_id UUID         REFERENCES delivery_addresses(id),
    has_shippable       BOOLEAN      NOT NULL DEFAULT false,
    is_buy_now          BOOLEAN      NOT NULL DEFAULT false,
    close_reason        VARCHAR(64),
    idempotency_key     VARCHAR(128),
    paid_at             TIMESTAMPTZ,
    closed_at           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user ON orders (user_id, created_at DESC);
CREATE INDEX idx_orders_status ON orders (status);
CREATE UNIQUE INDEX idx_orders_idempotency ON orders (idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Order items
CREATE TABLE order_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID         NOT NULL REFERENCES orders(id),
    item_type           VARCHAR(32)  NOT NULL,
    item_id             UUID         NOT NULL,
    item_name           VARCHAR(256) NOT NULL,
    quantity            INTEGER      NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price          BIGINT       NOT NULL CHECK (unit_price >= 0),
    line_total          BIGINT       NOT NULL CHECK (line_total >= 0),
    currency            CHAR(3)      NOT NULL DEFAULT 'CNY',
    is_shippable        BOOLEAN      NOT NULL DEFAULT false,
    metadata            JSONB        NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order ON order_items (order_id);

-- Order status history (immutable)
CREATE TABLE order_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID         NOT NULL REFERENCES orders(id),
    old_status  VARCHAR(32),
    new_status  VARCHAR(32)  NOT NULL,
    actor_type  VARCHAR(32)  NOT NULL DEFAULT 'system',
    actor_id    UUID,
    reason_code VARCHAR(64),
    note        TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_history ON order_status_history (order_id, created_at);

-- Payment requests
CREATE TABLE payment_requests (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID         NOT NULL REFERENCES orders(id),
    amount              BIGINT       NOT NULL CHECK (amount > 0),
    currency            CHAR(3)      NOT NULL DEFAULT 'CNY',
    status              VARCHAR(32)  NOT NULL DEFAULT 'created',
    -- status: created, pending_confirmation, confirmed, expired, rejected, canceled
    merchant_order_ref  VARCHAR(128) NOT NULL UNIQUE,
    qr_payload          TEXT,
    expires_at          TIMESTAMPTZ  NOT NULL,
    confirmed_at        TIMESTAMPTZ,
    gateway_tx_id       VARCHAR(256),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_req_order ON payment_requests (order_id);
CREATE INDEX idx_payment_req_expires ON payment_requests (expires_at) WHERE status IN ('created', 'pending_confirmation');
CREATE UNIQUE INDEX idx_payment_req_gateway ON payment_requests (gateway_tx_id) WHERE gateway_tx_id IS NOT NULL;

-- Payments
CREATE TABLE payments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID         NOT NULL REFERENCES orders(id),
    payment_request_id  UUID         REFERENCES payment_requests(id),
    amount              BIGINT       NOT NULL CHECK (amount > 0),
    currency            CHAR(3)      NOT NULL DEFAULT 'CNY',
    status              VARCHAR(32)  NOT NULL DEFAULT 'pending',
    -- status: pending, confirmed, failed, refunded
    method              VARCHAR(32)  NOT NULL DEFAULT 'wechat_offline',
    gateway_tx_id       VARCHAR(256),
    callback_payload    JSONB,
    verified            BOOLEAN      NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_order ON payments (order_id);

-- Refunds
CREATE TABLE refunds (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID         NOT NULL REFERENCES orders(id),
    payment_id          UUID         REFERENCES payments(id),
    amount              BIGINT       NOT NULL CHECK (amount > 0),
    currency            CHAR(3)      NOT NULL DEFAULT 'CNY',
    status              VARCHAR(32)  NOT NULL DEFAULT 'pending',
    -- status: pending, processing, completed, failed
    reason              TEXT,
    initiated_by        UUID         REFERENCES users(id),
    gateway_refund_id   VARCHAR(256),
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_refunds_order ON refunds (order_id);

-- Invoices
CREATE TABLE invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID         NOT NULL REFERENCES orders(id),
    invoice_number  VARCHAR(64)  NOT NULL UNIQUE,
    amount          BIGINT       NOT NULL,
    currency        CHAR(3)      NOT NULL DEFAULT 'CNY',
    status          VARCHAR(32)  NOT NULL DEFAULT 'issued',
    issued_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- E-vouchers
CREATE TABLE e_vouchers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID         NOT NULL REFERENCES orders(id),
    code            VARCHAR(128) NOT NULL UNIQUE,
    value           BIGINT       NOT NULL,
    currency        CHAR(3)      NOT NULL DEFAULT 'CNY',
    status          VARCHAR(32)  NOT NULL DEFAULT 'active',
    -- status: active, redeemed, expired, canceled
    expires_at      TIMESTAMPTZ,
    redeemed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS e_vouchers;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS refunds;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS payment_requests;
DROP TABLE IF EXISTS order_status_history;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS carts;
