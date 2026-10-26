-- Product catalog v1: description, archive timestamp, optimistic-lock version,
-- normalized SKU/currency checks, and the keyset pagination index.

UPDATE catalog.products
SET sku = upper(btrim(sku)),
    currency = upper(btrim(currency))
WHERE sku IS DISTINCT FROM upper(btrim(sku))
   OR currency IS DISTINCT FROM upper(btrim(currency));

ALTER TABLE platform.stores
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE platform.stores
    ADD CONSTRAINT stores_name_not_blank_check
        CHECK (btrim(name) <> '');

ALTER TABLE catalog.products
    ADD COLUMN description TEXT,
    ADD COLUMN archived_at TIMESTAMPTZ,
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

ALTER TABLE catalog.products
    ADD CONSTRAINT products_name_not_blank_check
        CHECK (btrim(name) <> ''),
    ADD CONSTRAINT products_sku_normalized_check
        CHECK (sku = upper(btrim(sku)) AND sku <> ''),
    ADD CONSTRAINT products_sku_charset_check
        CHECK (sku ~ '^[A-Z0-9][A-Z0-9_-]{0,63}$'),
    ADD CONSTRAINT products_currency_check
        CHECK (currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT products_status_archive_check
        CHECK (
            (status = 'active' AND archived_at IS NULL)
            OR (status = 'archived' AND archived_at IS NOT NULL)
        );

CREATE OR REPLACE FUNCTION platform.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER stores_set_updated_at
    BEFORE UPDATE ON platform.stores
    FOR EACH ROW
    EXECUTE FUNCTION platform.set_updated_at();

CREATE TRIGGER products_set_updated_at
    BEFORE UPDATE ON catalog.products
    FOR EACH ROW
    EXECUTE FUNCTION platform.set_updated_at();

-- Matches list queries: store + status + keyset order, ignoring soft-deleted rows.
CREATE INDEX products_store_status_created_idx
    ON catalog.products (store_id, status, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
