-- name: CreateCourier :one
INSERT INTO couriers (name)
VALUES ($1)
RETURNING id, name, created_at;

-- name: GetCourier :one
SELECT id, name, created_at
FROM couriers
WHERE id = $1;

-- name: ListCouriers :many
SELECT id, name, created_at
FROM couriers
ORDER BY name ASC;

-- name: UpdateCourier :one
UPDATE couriers
SET name = $2
WHERE id = $1
RETURNING id, name, created_at;

-- name: DeleteCourier :exec
DELETE FROM couriers
WHERE id = $1;

-- name: CreateCourierLocation :one
INSERT INTO courier_locations (courier_id, location_name, delivery_charge)
VALUES ($1, $2, $3)
RETURNING id, courier_id, location_name, delivery_charge, created_at;

-- name: GetCourierLocation :one
SELECT id, courier_id, location_name, delivery_charge, created_at
FROM courier_locations
WHERE id = $1 AND courier_id = $2;

-- name: ListCourierLocations :many
SELECT id, courier_id, location_name, delivery_charge, created_at
FROM courier_locations
WHERE courier_id = $1
ORDER BY location_name ASC;

-- name: UpdateCourierLocation :one
UPDATE courier_locations
SET location_name = $3,
    delivery_charge = $4
WHERE id = $1 AND courier_id = $2
RETURNING id, courier_id, location_name, delivery_charge, created_at;

-- name: DeleteCourierLocation :exec
DELETE FROM courier_locations
WHERE id = $1 AND courier_id = $2;