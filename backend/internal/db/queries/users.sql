-- name: GetUserByPhone :one
SELECT
    id,
    company_id,
    name,
    phone,
    password_hash,
    role,
    is_active,
    created_at
FROM users
WHERE phone = $1
LIMIT 1;

-- name: GetUserAuthContext :one
SELECT id, company_id, role, is_active
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetSetupStatus :one
SELECT EXISTS (
    SELECT 1 FROM users
) AS has_users;

-- name: CreateUser :one
INSERT INTO users (
    name,
    phone,
    password_hash,
    role,
    company_id
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING
    id,
    company_id,
    name,
    phone,
    password_hash,
    role,
    is_active,
    created_at;

-- name: ListUsers :many
SELECT id, company_id, name, phone, role, is_active,
             invitation_token_hash IS NOT NULL
             AND invitation_expires_at > NOW() AS invitation_pending,
             created_at
FROM users
WHERE company_id = $1
ORDER BY created_at ASC;

-- name: CreateInvitedUser :one
INSERT INTO users (
        name,
        phone,
        password_hash,
        role,
        is_active,
        invitation_token_hash,
        invitation_expires_at,
        company_id
)
    VALUES ($1, $2, $3, $4, FALSE, $5, $6, $7)
RETURNING id, name, phone, role, is_active, invitation_expires_at, created_at;

-- name: GetInvitation :one
SELECT id, company_id, name, phone, role
FROM users
WHERE invitation_token_hash = $1
    AND is_active = FALSE
    AND invitation_expires_at > NOW()
LIMIT 1;

-- name: UpdateInvitation :one
UPDATE users
SET invitation_token_hash = $2,
        invitation_expires_at = $3,
        is_active = FALSE
WHERE id = $1
    AND company_id = $4
    AND is_active = FALSE
RETURNING id, name, phone, role, is_active, invitation_expires_at, created_at;

-- name: RevokeInvitation :execresult
UPDATE users
SET invitation_token_hash = NULL,
        invitation_expires_at = NULL
WHERE id = $1
    AND company_id = $2
    AND is_active = FALSE;

-- name: AcceptInvitation :one
UPDATE users
SET password_hash = $2,
        is_active = TRUE,
        invitation_token_hash = NULL,
        invitation_expires_at = NULL
WHERE id = $1
    AND company_id = $4
    AND invitation_token_hash = $3
    AND is_active = FALSE
    AND invitation_expires_at > NOW()
RETURNING id, name, phone, role, is_active, created_at;

-- name: UpdateUserActive :one
UPDATE users
SET is_active = $2
WHERE id = $1
    AND company_id = $3
RETURNING id, name, phone, role, is_active, created_at;

-- name: ResetUserPassword :one
UPDATE users
SET password_hash = $2
WHERE id = $1
    AND company_id = $3
RETURNING id, name, phone, role, is_active, created_at;