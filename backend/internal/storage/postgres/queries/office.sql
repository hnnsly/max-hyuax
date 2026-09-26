-- name: InsertMaintenanceAlert :exec
INSERT INTO maintenance_alerts (id, house_id, category, title, description, starts_at, ends_at, created_by, created_at)
VALUES (@id, @house_id, @category, @title, @description, @starts_at, @ends_at, @created_by, @created_at);

-- name: ListActiveMaintenanceByHouse :many
SELECT id::text AS id, house_id, category, title, description, starts_at, ends_at, created_by, created_at
FROM maintenance_alerts
WHERE house_id = @house_id
  AND ends_at > @now
ORDER BY starts_at, id;

-- name: ListMaintenanceByOrg :many
SELECT m.id::text AS id, m.house_id, h.address, m.category, m.title, m.description, m.starts_at, m.ends_at, m.created_by, m.created_at
FROM maintenance_alerts m
JOIN houses h ON h.id = m.house_id
WHERE h.organization_id = @org_id
ORDER BY m.ends_at DESC, m.starts_at DESC
LIMIT @max_rows;

-- name: DeleteMaintenanceAlert :exec
DELETE FROM maintenance_alerts WHERE id = @id;

-- name: InsertAppointment :exec
INSERT INTO appointments (id, house_id, user_id, specialist, topic, slot_at, status, created_at)
VALUES (@id, @house_id, @user_id, @specialist, @topic, @slot_at, @status, @created_at);

-- name: GetAppointment :one
SELECT a.id::text AS id, a.house_id, h.address, a.user_id, u.first_name AS user_name, a.specialist, a.topic, a.slot_at, a.status, a.created_at
FROM appointments a
JOIN houses h ON h.id = a.house_id
JOIN users u ON u.id = a.user_id
WHERE a.id = @id;

-- name: CancelAppointment :exec
UPDATE appointments SET status = 'cancelled' WHERE id = @id;

-- name: ListUserAppointments :many
SELECT a.id::text AS id, a.house_id, h.address, a.user_id, u.first_name AS user_name, a.specialist, a.topic, a.slot_at, a.status, a.created_at
FROM appointments a
JOIN houses h ON h.id = a.house_id
JOIN users u ON u.id = a.user_id
WHERE a.user_id = @user_id
ORDER BY (a.status = 'cancelled'), a.slot_at
LIMIT @max_rows;

-- name: ListOrgAppointments :many
SELECT a.id::text AS id, a.house_id, h.address, a.user_id, u.first_name AS user_name, a.specialist, a.topic, a.slot_at, a.status, a.created_at
FROM appointments a
JOIN houses h ON h.id = a.house_id
JOIN users u ON u.id = a.user_id
WHERE h.organization_id = @org_id
ORDER BY (a.status = 'cancelled'), a.slot_at
LIMIT @max_rows;
