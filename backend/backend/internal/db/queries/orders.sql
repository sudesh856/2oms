-- name: GetOrderForAdmin :one
SELECT
    o.id,
    o.customer_id,
    o.source,
    o.status,
    o.courier_id,
    o.location_id,
    o.address,
    o.cod_amount,
    o.is_store_visit,
    o.created_by,
    o.created_at,
    o.updated_at
FROM orders o
WHERE o.id = $1;

-- name: GetOrderForStaff :one
SELECT
    o.id,
    o.customer_id,
    o.source,
    o.status,
    o.courier_id,
    o.location_id,
    o.address,
    o.is_store_visit,
    o.created_by,
    o.created_at,
    o.updated_at
FROM orders o
WHERE o.id = $1;