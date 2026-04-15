-- +goose Up
CREATE TABLE delivery_addresses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL REFERENCES users(id),
    label           VARCHAR(64)  NOT NULL DEFAULT '',
    recipient_name  VARCHAR(128) NOT NULL,
    phone           VARCHAR(32)  NOT NULL,
    line1           VARCHAR(256) NOT NULL,
    line2           VARCHAR(256) NOT NULL DEFAULT '',
    city            VARCHAR(128) NOT NULL,
    state           VARCHAR(128) NOT NULL DEFAULT '',
    postal_code     VARCHAR(32)  NOT NULL DEFAULT '',
    country_code    CHAR(2)      NOT NULL DEFAULT 'CN',
    is_default      BOOLEAN      NOT NULL DEFAULT false,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Only one default address per user among active addresses
CREATE UNIQUE INDEX idx_address_default_per_user
    ON delivery_addresses (user_id)
    WHERE is_default = true AND deleted_at IS NULL;

CREATE INDEX idx_address_user ON delivery_addresses (user_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS delivery_addresses;
