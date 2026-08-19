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
JOIN customers c ON c.id = o.customer_id
WHERE (
        sqlc.arg(search)::text = ''
        OR o.id::text ILIKE '%' || sqlc.arg(search)::text || '%'
        OR c.name ILIKE '%' || sqlc.arg(search)::text || '%'
        OR c.phone ILIKE '%' || sqlc.arg(search)::text || '%'
)
    AND (sqlc.arg(status)::text = '' OR o.status::text = sqlc.arg(status)::text)
    AND (sqlc.arg(from_date)::timestamptz IS NULL OR o.created_at >= sqlc.arg(from_date)::timestamptz)
    AND (sqlc.arg(to_date)::timestamptz IS NULL OR o.created_at < sqlc.arg(to_date)::timestamptz)
    AND (sqlc.arg(courier_id)::uuid IS NULL OR o.courier_id = sqlc.arg(courier_id)::uuid)
    AND (sqlc.arg(source)::text = '' OR o.source::text = sqlc.arg(source)::text)
ORDER BY o.created_at DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);


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
JOIN customers c ON c.id = o.customer_id
WHERE (
        sqlc.arg(search)::text = ''
        OR o.id::text ILIKE '%' || sqlc.arg(search)::text || '%'
        OR c.name ILIKE '%' || sqlc.arg(search)::text || '%'
        OR c.phone ILIKE '%' || sqlc.arg(search)::text || '%'
)
    AND (sqlc.arg(status)::text = '' OR o.status::text = sqlc.arg(status)::text)
    AND (sqlc.arg(from_date)::timestamptz IS NULL OR o.created_at >= sqlc.arg(from_date)::timestamptz)
    AND (sqlc.arg(to_date)::timestamptz IS NULL OR o.created_at < sqlc.arg(to_date)::timestamptz)
    AND (sqlc.arg(courier_id)::uuid IS NULL OR o.courier_id = sqlc.arg(courier_id)::uuid)
    AND (sqlc.arg(source)::text = '' OR o.source::text = sqlc.arg(source)::text)
ORDER BY o.created_at DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: ListOrderStatusHistory :many
SELECT
        sh.id,
        sh.order_id,
        sh.from_status,
        sh.to_status,
        sh.changed_by,
        sh.changed_at,
        u.name AS changed_by_name,
        u.phone AS changed_by_phone
FROM status_history sh
JOIN users u ON u.id = sh.changed_by
WHERE sh.order_id = $1
ORDER BY sh.changed_at ASC;



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