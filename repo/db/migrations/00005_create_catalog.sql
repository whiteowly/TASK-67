-- +goose Up

-- Program sessions (scheduled session offerings)
CREATE TABLE program_sessions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title                   VARCHAR(256)  NOT NULL,
    description             TEXT          NOT NULL DEFAULT '',
    short_description       VARCHAR(512)  NOT NULL DEFAULT '',
    category                VARCHAR(64),
    instructor_name         VARCHAR(128),
    tags                    TEXT[]        DEFAULT '{}',
    start_at                TIMESTAMPTZ   NOT NULL,
    end_at                  TIMESTAMPTZ   NOT NULL,
    seat_capacity           INTEGER       NOT NULL CHECK (seat_capacity >= 0),
    price_minor_units       BIGINT        NOT NULL DEFAULT 0 CHECK (price_minor_units >= 0),
    currency                CHAR(3)       NOT NULL DEFAULT 'CNY',
    registration_open_at    TIMESTAMPTZ,
    registration_close_at   TIMESTAMPTZ,
    requires_approval       BOOLEAN       NOT NULL DEFAULT false,
    allows_waitlist         BOOLEAN       NOT NULL DEFAULT true,
    status                  VARCHAR(32)   NOT NULL DEFAULT 'draft',
    -- status: draft, published, canceled, archived
    location                VARCHAR(256),
    created_by              UUID          REFERENCES users(id),
    deleted_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_session_times CHECK (start_at < end_at),
    CONSTRAINT chk_session_reg_close CHECK (
        registration_close_at IS NULL OR registration_close_at <= start_at
    )
);

-- Full-text search index
CREATE INDEX idx_program_sessions_search ON program_sessions
    USING GIN (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(short_description, '') || ' ' || coalesce(category, '') || ' ' || coalesce(instructor_name, '')));

CREATE INDEX idx_program_sessions_status ON program_sessions (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_program_sessions_start ON program_sessions (start_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_program_sessions_category ON program_sessions (category) WHERE deleted_at IS NULL;

-- Seat inventory (authoritative count, updated transactionally)
CREATE TABLE session_seat_inventory (
    session_id       UUID PRIMARY KEY REFERENCES program_sessions(id),
    total_seats      INTEGER NOT NULL CHECK (total_seats >= 0),
    reserved_seats   INTEGER NOT NULL DEFAULT 0 CHECK (reserved_seats >= 0),
    available_seats  INTEGER GENERATED ALWAYS AS (total_seats - reserved_seats) STORED,
    version          INTEGER NOT NULL DEFAULT 1,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_no_oversell CHECK (reserved_seats <= total_seats)
);

-- Products (purchasable merchandise)
CREATE TABLE products (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(256)  NOT NULL,
    description         TEXT          NOT NULL DEFAULT '',
    short_description   VARCHAR(512)  NOT NULL DEFAULT '',
    category            VARCHAR(64),
    sku                 VARCHAR(64)   UNIQUE,
    price_minor_units   BIGINT        NOT NULL CHECK (price_minor_units >= 0),
    currency            CHAR(3)       NOT NULL DEFAULT 'CNY',
    is_shippable        BOOLEAN       NOT NULL DEFAULT true,
    status              VARCHAR(32)   NOT NULL DEFAULT 'draft',
    -- status: draft, published, discontinued, archived
    image_url           TEXT,
    tags                TEXT[]        DEFAULT '{}',
    created_by          UUID          REFERENCES users(id),
    deleted_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_search ON products
    USING GIN (to_tsvector('english', coalesce(name, '') || ' ' || coalesce(short_description, '') || ' ' || coalesce(category, '')));

CREATE INDEX idx_products_status ON products (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_category ON products (category) WHERE deleted_at IS NULL;

-- Product inventory
CREATE TABLE product_inventory (
    product_id   UUID PRIMARY KEY REFERENCES products(id),
    stock_qty    INTEGER NOT NULL DEFAULT 0 CHECK (stock_qty >= 0),
    version      INTEGER NOT NULL DEFAULT 1,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS product_inventory;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS session_seat_inventory;
DROP TABLE IF EXISTS program_sessions;
