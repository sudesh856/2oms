-- name: GetDashboardCounts :one
SELECT
    COUNT(*) FILTER (WHERE o.created_at >= $1::timestamptz AND o.created_at < $2::timestamptz)::int AS today_orders,
    COUNT(*) FILTER (WHERE o.status = 'confirmed')::int AS pending_confirmations,
    COUNT(*) FILTER (WHERE o.status IN ('follow_up', 'hold', 'redirected', 'cancelled', 'returned'))::int AS problem_orders,
    COUNT(*)::int AS total_orders
FROM orders o
WHERE o.company_id = $3::uuid;

-- name: ListDashboardStatusCounts :many
SELECT status, COUNT(*)::int AS count
FROM orders
WHERE company_id = $1::uuid
GROUP BY status
ORDER BY status;

-- name: ListDashboardCourierCounts :many
SELECT c.name AS courier_name, COUNT(*)::int AS count
FROM orders o
LEFT JOIN couriers c ON c.id = o.courier_id
WHERE o.created_at >= $1::timestamptz AND o.created_at < $2::timestamptz
    AND o.company_id = $3::uuid
GROUP BY c.name
ORDER BY c.name;

-- name: ListDashboardFollowUpsDue :many
SELECT
    f.id,
    f.order_id,
    f.next_action,
    f.next_action_date,
    f.note,
    c.name AS customer_name,
    c.phone AS customer_phone
FROM follow_ups f
JOIN orders o ON o.id = f.order_id
JOIN customers c ON c.id = o.customer_id
WHERE f.next_action_date = $1::date
    AND f.company_id = $2::uuid
ORDER BY f.created_at ASC;

-- name: ListProblemOrdersForAdmin :many
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.cod_amount, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o
WHERE o.status IN ('follow_up', 'hold', 'redirected', 'cancelled', 'returned')
    AND o.company_id = $2::uuid
ORDER BY o.created_at DESC
LIMIT $1;

-- name: ListProblemOrdersForStaff :many
SELECT o.id, o.customer_id, o.source, o.status, o.courier_id, o.location_id,
       o.address, o.is_store_visit, o.created_by, o.created_at,
       o.updated_at, o.is_legacy
FROM orders o
WHERE o.status IN ('follow_up', 'hold', 'redirected', 'cancelled', 'returned')
    AND o.company_id = $2::uuid
ORDER BY o.created_at DESC
LIMIT $1;
