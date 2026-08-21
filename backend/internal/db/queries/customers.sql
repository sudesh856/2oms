-- name: CreateCustomer :one
INSERT INTO customers (
    phone,
    phone2,
    name,
    address,
    company_id
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetCustomerByID :one
SELECT *
FROM customers
WHERE id = $1 AND company_id = $2
LIMIT 1;

-- name: GetCustomerByPhone :one
SELECT *
FROM customers
WHERE phone = $1 AND company_id = $2
LIMIT 1;

-- name: ListCustomers :many
SELECT *
FROM customers
WHERE company_id = $1
  AND ($2 = ''
    OR phone ILIKE '%' || $2 || '%'
    OR name ILIKE '%' || $2 || '%')
ORDER BY created_at DESC
LIMIT $3
OFFSET $4;

-- name: UpdateCustomer :one
UPDATE customers
SET
    phone = $2,
    phone2 = $3,
    name = $4,
    address = $5
WHERE id = $1 AND company_id = $6
RETURNING *;
