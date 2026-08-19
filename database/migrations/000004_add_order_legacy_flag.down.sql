DROP INDEX IF EXISTS idx_orders_is_legacy;

ALTER TABLE orders
DROP COLUMN IF EXISTS is_legacy;
