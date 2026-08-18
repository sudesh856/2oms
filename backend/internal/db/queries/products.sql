-- name: CreateProduct :one
INSERT INTO products (
    name,
    price,
    available_qty,
    warehouse_qty
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    id,
    name,
    price,
    available_qty,
    warehouse_qty,
    created_at;


-- name: GetProductByID :one
SELECT
    id,
    name,
    price,
    available_qty,
    warehouse_qty,
    created_at
FROM products
WHERE id = $1
LIMIT 1;


-- name: ListProducts :many
SELECT
    id,
    name,
    price,
    available_qty,
    warehouse_qty,
    created_at
FROM products
WHERE $1 = ''
   OR name ILIKE '%' || $1 || '%'
ORDER BY name ASC;


-- name: UpdateProduct :one
UPDATE products
SET
    name = $2,
    price = $3,
    available_qty = $4,
    warehouse_qty = $5
WHERE id = $1
RETURNING
    id,
    name,
    price,
    available_qty,
    warehouse_qty,
    created_at;

-- name: DecreaseProductAvailableQty :one
UPDATE products
SET available_qty = available_qty - $2
WHERE id = $1
  AND available_qty >= $2
RETURNING
    id,
    name,
    price,
    available_qty,
    warehouse_qty,
    created_at;