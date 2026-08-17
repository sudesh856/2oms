-- name: GetUserByPhone :one
SELECT
    id,
    name,
    phone,
    password_hash,
    role,
    created_at
FROM users
WHERE phone = $1
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
    name,
    phone,
    password_hash,
    role
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
    phone,
    password_hash,
    role,
    created_at;