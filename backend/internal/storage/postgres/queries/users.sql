-- name: GetUser :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByMaxID :one
SELECT * FROM users WHERE max_user_id = @max_user_id;

-- name: GetUserByDemoKey :one
SELECT * FROM users WHERE demo_key = @demo_key;

-- name: InsertUser :one
INSERT INTO users (max_user_id, first_name, role)
VALUES (@max_user_id, @first_name, @role)
RETURNING *;

-- name: UpdateUser :exec
UPDATE users
SET max_user_id     = @max_user_id,
    first_name      = @first_name,
    phone           = @phone,
    house_id        = NULLIF(@house_id::text, ''),
    consent_version = @consent_version,
    consent_at      = @consent_at,
    deleted_at      = @deleted_at
WHERE id = @id;

-- name: MarkUpdateProcessed :execrows
INSERT INTO processed_updates (key) VALUES (@key) ON CONFLICT DO NOTHING;
