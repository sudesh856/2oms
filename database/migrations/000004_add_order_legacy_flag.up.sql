ALTER TABLE orders
ADD COLUMN is_legacy BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_orders_is_legacy ON orders(is_legacy);
