DROP INDEX IF EXISTS idx_orders_status_updated_at;

ALTER TABLE orders DROP COLUMN updated_at;
