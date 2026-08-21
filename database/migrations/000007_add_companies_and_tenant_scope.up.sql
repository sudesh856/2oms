CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE users ADD COLUMN company_id UUID;
ALTER TABLE customers ADD COLUMN company_id UUID;
ALTER TABLE products ADD COLUMN company_id UUID;
ALTER TABLE couriers ADD COLUMN company_id UUID;
ALTER TABLE courier_locations ADD COLUMN company_id UUID;
ALTER TABLE orders ADD COLUMN company_id UUID;
ALTER TABLE order_items ADD COLUMN company_id UUID;
ALTER TABLE follow_ups ADD COLUMN company_id UUID;
ALTER TABLE status_history ADD COLUMN company_id UUID;

INSERT INTO companies (name)
VALUES ('Default Company');

WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE users SET company_id = initial_company.id FROM initial_company WHERE users.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE customers SET company_id = initial_company.id FROM initial_company WHERE customers.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE products SET company_id = initial_company.id FROM initial_company WHERE products.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE couriers SET company_id = initial_company.id FROM initial_company WHERE couriers.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE courier_locations SET company_id = initial_company.id FROM initial_company WHERE courier_locations.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE orders SET company_id = initial_company.id FROM initial_company WHERE orders.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE order_items SET company_id = initial_company.id FROM initial_company WHERE order_items.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE follow_ups SET company_id = initial_company.id FROM initial_company WHERE follow_ups.company_id IS NULL;
WITH initial_company AS (SELECT id FROM companies WHERE name = 'Default Company')
UPDATE status_history SET company_id = initial_company.id FROM initial_company WHERE status_history.company_id IS NULL;

ALTER TABLE users ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE customers ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE products ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE couriers ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE courier_locations ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE orders ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE order_items ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE follow_ups ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE status_history ALTER COLUMN company_id SET NOT NULL;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_phone_key;
ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_phone_key;
ALTER TABLE couriers DROP CONSTRAINT IF EXISTS couriers_name_key;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_name_key;

ALTER TABLE users ADD CONSTRAINT users_phone_key UNIQUE (phone);
ALTER TABLE customers ADD CONSTRAINT customers_company_phone_key UNIQUE (company_id, phone);
ALTER TABLE couriers ADD CONSTRAINT couriers_company_name_key UNIQUE (company_id, name);
ALTER TABLE products ADD CONSTRAINT products_company_name_key UNIQUE (company_id, name);
ALTER TABLE courier_locations ADD CONSTRAINT courier_locations_company_id_key UNIQUE (id, company_id);
ALTER TABLE customers ADD CONSTRAINT customers_company_id_key UNIQUE (id, company_id);
ALTER TABLE products ADD CONSTRAINT products_company_id_key UNIQUE (id, company_id);
ALTER TABLE couriers ADD CONSTRAINT couriers_company_id_key UNIQUE (id, company_id);
ALTER TABLE users ADD CONSTRAINT users_company_id_key UNIQUE (id, company_id);
ALTER TABLE orders ADD CONSTRAINT orders_company_id_key UNIQUE (id, company_id);

ALTER TABLE courier_locations DROP CONSTRAINT IF EXISTS courier_locations_courier_id_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_customer_id_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_courier_id_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_location_id_fkey;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_created_by_fkey;
ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_order_id_fkey;
ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_product_id_fkey;
ALTER TABLE follow_ups DROP CONSTRAINT IF EXISTS follow_ups_order_id_fkey;
ALTER TABLE follow_ups DROP CONSTRAINT IF EXISTS follow_ups_assigned_to_fkey;
ALTER TABLE status_history DROP CONSTRAINT IF EXISTS status_history_order_id_fkey;
ALTER TABLE status_history DROP CONSTRAINT IF EXISTS status_history_changed_by_fkey;

ALTER TABLE courier_locations ADD CONSTRAINT courier_locations_company_courier_fkey
    FOREIGN KEY (courier_id, company_id) REFERENCES couriers (id, company_id);
ALTER TABLE orders ADD CONSTRAINT orders_company_customer_fkey
    FOREIGN KEY (customer_id, company_id) REFERENCES customers (id, company_id);
ALTER TABLE orders ADD CONSTRAINT orders_company_courier_fkey
    FOREIGN KEY (courier_id, company_id) REFERENCES couriers (id, company_id);
ALTER TABLE orders ADD CONSTRAINT orders_company_location_fkey
    FOREIGN KEY (location_id, company_id) REFERENCES courier_locations (id, company_id);
ALTER TABLE orders ADD CONSTRAINT orders_company_creator_fkey
    FOREIGN KEY (created_by, company_id) REFERENCES users (id, company_id);
ALTER TABLE order_items ADD CONSTRAINT order_items_company_order_fkey
    FOREIGN KEY (order_id, company_id) REFERENCES orders (id, company_id) ON DELETE CASCADE;
ALTER TABLE order_items ADD CONSTRAINT order_items_company_product_fkey
    FOREIGN KEY (product_id, company_id) REFERENCES products (id, company_id);
ALTER TABLE follow_ups ADD CONSTRAINT follow_ups_company_order_fkey
    FOREIGN KEY (order_id, company_id) REFERENCES orders (id, company_id) ON DELETE CASCADE;
ALTER TABLE follow_ups ADD CONSTRAINT follow_ups_company_assignee_fkey
    FOREIGN KEY (assigned_to, company_id) REFERENCES users (id, company_id);
ALTER TABLE status_history ADD CONSTRAINT status_history_company_order_fkey
    FOREIGN KEY (order_id, company_id) REFERENCES orders (id, company_id) ON DELETE CASCADE;
ALTER TABLE status_history ADD CONSTRAINT status_history_company_changer_fkey
    FOREIGN KEY (changed_by, company_id) REFERENCES users (id, company_id);

CREATE INDEX idx_users_company_id ON users(company_id);
CREATE INDEX idx_customers_company_id ON customers(company_id);
CREATE INDEX idx_products_company_id ON products(company_id);
CREATE INDEX idx_couriers_company_id ON couriers(company_id);
CREATE INDEX idx_courier_locations_company_id ON courier_locations(company_id);
CREATE INDEX idx_orders_company_id ON orders(company_id);
CREATE INDEX idx_order_items_company_id ON order_items(company_id);
CREATE INDEX idx_follow_ups_company_id ON follow_ups(company_id);
CREATE INDEX idx_status_history_company_id ON status_history(company_id);