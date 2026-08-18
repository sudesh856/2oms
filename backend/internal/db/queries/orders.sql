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

-- name: UpdateOrderStatus :one
UPDATE orders
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1
  AND status = $3
RETURNING
    id,
    customer_id,
    source,
    status,
    courier_id,
    location_id,
    address,
    cod_amount,
    is_store_visit,
    created_by,
    created_at,
    updated_at;

-- name: CreateStatusHistory :one
INSERT INTO status_history (
    order_id,
    from_status,
    to_status,
    changed_by
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    id,
    order_id,
    from_status,
    to_status,
    changed_by,
    changed_at;


-- name: ListOrdersForAdmin :many
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
ORDER BY o.created_at DESC;


-- name: ListOrdersForStaff :many
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
ORDER BY o.created_at DESC;



-- name: CreateOrderForAdmin :one
INSERT INTO orders (
    customer_id,
    source,
    status,
    address,
    cod_amount,
    is_store_visit,
    created_by
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING
    id,
    customer_id,
    source,
    status,
    courier_id,
    location_id,
    address,
    cod_amount,
    is_store_visit,
    created_by,
    created_at,
    updated_at;


-- name: CreateOrderForStaff :one
INSERT INTO orders (
    customer_id,
    source,
    status,
    address,
    is_store_visit,
    created_by
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    customer_id,
    source,
    status,
    courier_id,
    location_id,
    address,
    is_store_visit,
    created_by,
    created_at,
    updated_at;