-- name: GetOrderForAdmin :one
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.cod_amount, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o WHERE o.id = $1 AND o.company_id = $2;

-- name: GetOrderForStaff :one
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o WHERE o.id = $1 AND o.company_id = $2;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = NOW()
WHERE id = $1 AND status = $3 AND company_id = $4
RETURNING id, customer_id, source, status, courier_id, location_id, address,
          cod_amount, is_store_visit, created_by, created_at, updated_at, is_legacy;

-- name: CreateStatusHistory :one
INSERT INTO status_history (order_id, from_status, to_status, changed_by, company_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, order_id, from_status, to_status, changed_by, changed_at;

-- name: ListOrdersForAdmin :many
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.cod_amount, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o
JOIN customers c ON c.id = o.customer_id
WHERE ($1::text = '' OR o.id::text ILIKE '%' || $1::text || '%' OR c.name ILIKE '%' || $1::text || '%' OR c.phone ILIKE '%' || $1::text || '%')
  AND ($2::text = '' OR o.status::text = $2::text)
  AND ($3::timestamptz IS NULL OR o.created_at >= $3::timestamptz)
  AND ($4::timestamptz IS NULL OR o.created_at < $4::timestamptz)
  AND ($5::uuid IS NULL OR o.courier_id = $5::uuid)
  AND ($6::text = '' OR o.source::text = $6::text)
  AND ($7::uuid IS NULL OR o.customer_id = $7::uuid)
  AND o.company_id = $10::uuid
ORDER BY o.created_at DESC
LIMIT $9 OFFSET $8;

-- name: ListOrdersForStaff :many
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o
JOIN customers c ON c.id = o.customer_id
WHERE ($1::text = '' OR o.id::text ILIKE '%' || $1::text || '%' OR c.name ILIKE '%' || $1::text || '%' OR c.phone ILIKE '%' || $1::text || '%')
  AND ($2::text = '' OR o.status::text = $2::text)
  AND ($3::timestamptz IS NULL OR o.created_at >= $3::timestamptz)
  AND ($4::timestamptz IS NULL OR o.created_at < $4::timestamptz)
  AND ($5::uuid IS NULL OR o.courier_id = $5::uuid)
  AND ($6::text = '' OR o.source::text = $6::text)
  AND ($7::uuid IS NULL OR o.customer_id = $7::uuid)
  AND o.company_id = $10::uuid
ORDER BY o.created_at DESC
LIMIT $9 OFFSET $8;

-- name: ListCustomerOrdersForAdmin :many
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.cod_amount, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o WHERE o.customer_id = $1 AND o.company_id = $2 ORDER BY o.created_at DESC;

-- name: ListCustomerOrdersForStaff :many
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o WHERE o.customer_id = $1 AND o.company_id = $2 ORDER BY o.created_at DESC;

-- name: CreateOrderForAdmin :one
INSERT INTO orders (customer_id, source, status, address, cod_amount, is_store_visit, courier_id, location_id, created_by, company_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, customer_id, source, status, courier_id, location_id, address,
          cod_amount, is_store_visit, created_by, created_at, updated_at, is_legacy;

-- name: CreateOrderForStaff :one
INSERT INTO orders (customer_id, source, status, address, is_store_visit, courier_id, location_id, created_by, company_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, customer_id, source, status, courier_id, location_id, address,
          is_store_visit, created_by, created_at, updated_at, is_legacy;

-- name: UpdateOrderCourierAndLocation :one
UPDATE orders
SET courier_id = $2,
    location_id = $3,
    updated_at = NOW()
WHERE id = $1 AND company_id = $4
RETURNING id, customer_id, source, status, courier_id, location_id, address,
          cod_amount, is_store_visit, created_by, created_at, updated_at, is_legacy;

-- name: CreateLegacyOrder :one
INSERT INTO orders (customer_id, source, status, address, cod_amount, is_store_visit,
                    courier_id, location_id, created_by, created_at, is_legacy, company_id, legacy_source_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, TRUE, $11, $12)
ON CONFLICT (company_id, legacy_source_key) WHERE legacy_source_key IS NOT NULL DO NOTHING
RETURNING id, customer_id, source, status, courier_id, location_id, address,
          cod_amount, is_store_visit, created_by, created_at, updated_at, is_legacy;

