CREATE SCHEMA IF NOT EXISTS platform;
CREATE SCHEMA IF NOT EXISTS catalog;

CREATE TABLE platform.stores (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE catalog.products (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    store_id UUID NOT NULL
        REFERENCES platform.stores(id),
    sku TEXT NOT NULL,
    name TEXT NOT NULL,
    price NUMERIC(19, 4) NOT NULL
        CHECK (price >= 0),
    currency CHAR(3) NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX products_store_sku_unique
    ON catalog.products (store_id, sku)
    WHERE deleted_at IS NULL;
