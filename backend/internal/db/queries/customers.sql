-- name: CreateCustomer :one
INSERT INTO customers (
    phone,
    phone2,
    name,
    address
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetCustomerByID :one
SELECT *
FROM customers
WHERE id = $1
LIMIT 1;

-- name: GetCustomerByPhone :one
SELECT *
FROM customers
WHERE phone = $1
LIMIT 1;

-- name: ListCustomers :many
SELECT *
FROM customers
WHERE $1 = ''
   OR phone ILIKE '%' || $1 || '%'
   OR name ILIKE '%' || $1 || '%'
ORDER BY created_at DESC
LIMIT $2
OFFSET $3;

-- name: UpdateCustomer :one
UPDATE customers
SET
    phone = $2,
    phone2 = $3,
    name = $4,
    address = $5
WHERE id = $1
RETURNING *;
