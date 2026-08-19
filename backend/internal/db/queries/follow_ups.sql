-- name: CreateFollowUp :one
INSERT INTO follow_ups (
    order_id,
    attempt_no,
    next_action,
    preferred_day,
    next_action_date,
    note,
    assigned_to
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
    order_id,
    attempt_no,
    next_action,
    preferred_day,
    next_action_date,
    note,
    assigned_to,
    created_at;

-- name: GetLatestFollowUpAttempt :one
SELECT COALESCE(MAX(attempt_no), 0)::int
FROM follow_ups
WHERE order_id = $1;

-- name: ListFollowUps :many
SELECT
    f.id,
    f.order_id,
    f.attempt_no,
    f.next_action,
    f.preferred_day,
    f.next_action_date,
    f.note,
    f.assigned_to,
    f.created_at,
    o.status,
    o.customer_id,
    c.name AS customer_name,
    c.phone AS customer_phone,
    u.name AS assigned_to_name,
    u.phone AS assigned_to_phone
FROM follow_ups f
JOIN orders o ON o.id = f.order_id
JOIN customers c ON c.id = o.customer_id
LEFT JOIN users u ON u.id = f.assigned_to
WHERE (
    $1::date IS NULL
    OR f.next_action_date = $1::date
)
AND (
    $2::boolean = FALSE
    OR f.next_action = 'no_answer'
)
ORDER BY f.next_action_date NULLS LAST, f.created_at ASC;

-- name: DeleteFollowUp :exec
DELETE FROM follow_ups
WHERE id = $1;