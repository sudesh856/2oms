-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_id,
    product_id,
    quantity,
    price
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
    product_id,
    quantity,
    price;


-- name: ListOrderItems :many
SELECT
    id,
    order_id,
    product_id,
    quantity,
    price
FROM order_items
WHERE order_id = $1
ORDER BY id;