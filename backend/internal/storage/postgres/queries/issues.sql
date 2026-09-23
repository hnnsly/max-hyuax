-- name: InsertIssue :one
INSERT INTO issues (id, house_id, object_id, category, title, description, responsible_org_id,
                    status, status_at, status_comment, created_by, created_at, deadline_at)
VALUES (@id, @house_id, NULLIF(@object_id::text, ''), @category, @title, @description, @responsible_org_id,
        @status, @status_at, @status_comment, @created_by, @created_at, @deadline_at)
RETURNING number;

-- name: UpdateIssueState :exec
UPDATE issues
SET status = @status, status_at = @status_at, status_comment = @status_comment
WHERE id = @id;

-- name: GetIssue :one
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at
FROM issues
WHERE id = @id;

-- name: LockIssue :exec
SELECT 1 FROM issues WHERE id = @id FOR UPDATE;

-- name: ListHouseIssues :many
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at
FROM issues
WHERE house_id = @house_id
ORDER BY created_at DESC
LIMIT @max_rows;

-- name: FindSimilarIssues :many
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at
FROM issues
WHERE house_id = @house_id
  AND category = @category
  AND (@object_id::text = '' OR object_id IS NULL OR object_id = @object_id::text)
  AND status NOT IN ('done', 'rejected')
  AND created_at >= @since
ORDER BY (object_id = @object_id::text) DESC NULLS LAST, created_at DESC
LIMIT 5;

-- name: ListOrgQueue :many
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at
FROM issues
WHERE responsible_org_id = @org_id
ORDER BY status IN ('done', 'rejected'), deadline_at
LIMIT @max_rows;

-- name: ListParticipants :many
SELECT issue_id, user_id, joined_at
FROM issue_participants
WHERE issue_id = ANY (@issue_ids::uuid[])
ORDER BY joined_at, user_id;

-- name: InsertParticipant :exec
INSERT INTO issue_participants (issue_id, user_id, joined_at)
VALUES (@issue_id, @user_id, @joined_at)
ON CONFLICT DO NOTHING;

-- name: InsertEvent :exec
INSERT INTO issue_events (issue_id, kind, user_id, status, comment, at)
VALUES (@issue_id, @kind, NULLIF(@user_id::bigint, 0), @status, @comment, @at);
