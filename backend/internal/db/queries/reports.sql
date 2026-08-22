-- name: ListStaffPerformance :many
WITH staff AS (
    SELECT id, name, role
    FROM users
    WHERE company_id = $1::uuid
), follow_up_counts AS (
    SELECT assigned_to AS user_id, COUNT(*)::int AS count
    FROM follow_ups
    WHERE company_id = $1::uuid
    GROUP BY assigned_to
), status_counts AS (
    SELECT changed_by AS user_id,
        COUNT(*) FILTER (WHERE to_status = 'confirmed')::int AS confirmed_count,
        COUNT(*) FILTER (WHERE to_status = 'cancelled')::int AS cancelled_count
    FROM status_history
    WHERE company_id = $1::uuid
    GROUP BY changed_by
)
SELECT
    staff.id AS user_id,
    staff.name,
    staff.role,
    COALESCE(follow_up_counts.count, 0)::int AS calls_made,
    COALESCE(status_counts.confirmed_count, 0)::int AS orders_confirmed,
    COALESCE(status_counts.cancelled_count, 0)::int AS orders_cancelled,
    COALESCE(follow_up_counts.count, 0)::int AS follow_ups_logged
FROM staff
LEFT JOIN follow_up_counts ON follow_up_counts.user_id = staff.id
LEFT JOIN status_counts ON status_counts.user_id = staff.id
WHERE staff.role IN ('staff', 'admin', 'superadmin')
ORDER BY staff.name;

-- name: ListConfirmedCourierLocationCounts :many
WITH confirmed_orders AS (
    SELECT o.id, o.courier_id, o.location_id
    FROM orders o
    WHERE o.company_id = $1::uuid
        AND (o.status = 'confirmed' OR EXISTS (
            SELECT 1 FROM status_history sh
            WHERE sh.order_id = o.id
                AND sh.company_id = o.company_id
                AND sh.to_status = 'confirmed'
        ))
)
SELECT
    c.name AS courier_name,
    cl.location_name,
    COUNT(DISTINCT confirmed_orders.id)::int AS order_count,
    COALESCE(SUM(oi.quantity), 0)::int AS product_count
FROM confirmed_orders
JOIN couriers c ON c.id = confirmed_orders.courier_id
    AND c.company_id = $1::uuid
LEFT JOIN courier_locations cl ON cl.id = confirmed_orders.location_id
    AND cl.company_id = $1::uuid
LEFT JOIN order_items oi ON oi.order_id = confirmed_orders.id
    AND oi.company_id = $1::uuid
GROUP BY c.name, cl.location_name
ORDER BY c.name, cl.location_name;