ALTER TABLE status_history DROP CONSTRAINT IF EXISTS status_history_company_changer_fkey;
ALTER TABLE status_history DROP CONSTRAINT IF EXISTS status_history_company_order_fkey;
ALTER TABLE follow_ups DROP CONSTRAINT IF EXISTS follow_ups_company_assignee_fkey;
ALTER TABLE follow_ups DROP CONSTRAINT IF EXISTS follow_ups_company_order_fkey;
ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_company_product_fkey;
ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_company_order_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_company_creator_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_company_location_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_company_courier_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_company_customer_fkey;
ALTER TABLE courier_locations DROP CONSTRAINT IF EXISTS courier_locations_company_courier_fkey;

ALTER TABLE customers ADD CONSTRAINT customers_phone_key UNIQUE (phone);
ALTER TABLE couriers ADD CONSTRAINT couriers_name_key UNIQUE (name);
ALTER TABLE products ADD CONSTRAINT products_name_key UNIQUE (name);

ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_company_phone_key;
ALTER TABLE couriers DROP CONSTRAINT IF EXISTS couriers_company_name_key;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_company_name_key;

DROP INDEX IF EXISTS idx_status_history_company_id;
DROP INDEX IF EXISTS idx_follow_ups_company_id;
DROP INDEX IF EXISTS idx_order_items_company_id;
DROP INDEX IF EXISTS idx_orders_company_id;
DROP INDEX IF EXISTS idx_courier_locations_company_id;
DROP INDEX IF EXISTS idx_couriers_company_id;
DROP INDEX IF EXISTS idx_products_company_id;
DROP INDEX IF EXISTS idx_customers_company_id;
DROP INDEX IF EXISTS idx_users_company_id;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_company_id_key;
ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_company_id_key;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_company_id_key;
ALTER TABLE couriers DROP CONSTRAINT IF EXISTS couriers_company_id_key;
ALTER TABLE courier_locations DROP CONSTRAINT IF EXISTS courier_locations_company_id_key;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_company_id_key;

ALTER TABLE status_history DROP COLUMN company_id;
ALTER TABLE follow_ups DROP COLUMN company_id;
ALTER TABLE order_items DROP COLUMN company_id;
ALTER TABLE orders DROP COLUMN company_id;
ALTER TABLE courier_locations DROP COLUMN company_id;
ALTER TABLE couriers DROP COLUMN company_id;
ALTER TABLE products DROP COLUMN company_id;
ALTER TABLE customers DROP COLUMN company_id;
ALTER TABLE users DROP COLUMN company_id;
DROP TABLE companies;