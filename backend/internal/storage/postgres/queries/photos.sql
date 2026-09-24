-- name: InsertPhoto :exec
INSERT INTO issue_photos (id, issue_id, uploaded_by, width, height, size_bytes, created_at)
VALUES (@id, @issue_id, @uploaded_by, @width, @height, @size_bytes, @created_at);

-- name: ListIssuePhotos :many
SELECT id, issue_id, uploaded_by, width, height, size_bytes, created_at
FROM issue_photos
WHERE issue_id = @issue_id
ORDER BY created_at, id;

-- name: GetPhoto :one
SELECT id, issue_id, uploaded_by, width, height, size_bytes, created_at
FROM issue_photos
WHERE id = @id;
