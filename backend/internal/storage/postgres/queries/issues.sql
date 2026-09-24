-- name: InsertIssue :one
INSERT INTO issues (id, house_id, object_id, category, title, description, responsible_org_id,
                    status, status_at, status_comment, created_by, created_at, deadline_at)
VALUES (@id, @house_id, NULLIF(@object_id::text, ''), @category, @title, @description, @responsible_org_id,
        @status, @status_at, @status_comment, @created_by, @created_at, @deadline_at)
RETURNING number;

-- name: UpdateIssueState :exec
-- Срок меняется, когда жители возвращают заявку в работу: он считается заново по справочнику.
UPDATE issues
SET status = @status, status_at = @status_at, status_comment = @status_comment, overdue_at = @overdue_at,
    deadline_at = @deadline_at, reopened_at = @reopened_at
WHERE id = @id;

-- name: GetIssue :one
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at, overdue_at, reopened_at
FROM issues
WHERE id = @id;

-- name: LockIssue :exec
SELECT 1 FROM issues WHERE id = @id FOR UPDATE;

-- name: ListHouseIssues :many
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at, overdue_at, reopened_at
FROM issues
WHERE house_id = @house_id
ORDER BY created_at DESC
LIMIT @max_rows;

-- name: FindSimilarIssues :many
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at, overdue_at, reopened_at
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
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at, overdue_at, reopened_at
FROM issues
WHERE responsible_org_id = @org_id
ORDER BY status IN ('done', 'rejected'), deadline_at
LIMIT @max_rows;

-- name: ListParticipantIssues :many
SELECT i.id, i.number, i.house_id, COALESCE(i.object_id, '')::text AS object_id, i.category, i.title, i.description,
       i.responsible_org_id, i.status, i.status_at, i.status_comment, i.created_by, i.created_at, i.deadline_at, i.overdue_at, i.reopened_at
FROM issues i
JOIN issue_participants p ON p.issue_id = i.id
WHERE p.user_id = @user_id
ORDER BY i.status IN ('done', 'rejected'), i.created_at DESC
LIMIT @max_rows;

-- name: ListOverdueUnmarked :many
SELECT id, number, house_id, COALESCE(object_id, '')::text AS object_id, category, title, description,
       responsible_org_id, status, status_at, status_comment, created_by, created_at, deadline_at, overdue_at, reopened_at
FROM issues
WHERE overdue_at IS NULL
  AND status NOT IN ('done', 'rejected')
  AND deadline_at < @now
ORDER BY deadline_at
LIMIT @max_rows;

-- name: ListIssueEvents :many
SELECT kind, COALESCE(user_id, 0)::bigint AS user_id, status, comment, at
FROM issue_events
WHERE issue_id = @issue_id
ORDER BY at, id;

-- name: ListParticipants :many
SELECT issue_id, user_id, joined_at
FROM issue_participants
WHERE issue_id = ANY (@issue_ids::uuid[])
ORDER BY joined_at, user_id;

-- name: InsertParticipant :exec
INSERT INTO issue_participants (issue_id, user_id, joined_at)
VALUES (@issue_id, @user_id, @joined_at)
ON CONFLICT DO NOTHING;

-- name: ListCurrentAnswers :many
-- Ответы на текущее «выполнено»: у заявки в другом статусе их нет.
SELECT c.issue_id, c.user_id, c.fixed, c.done_at, c.at
FROM issue_confirmations c
JOIN issues i ON i.id = c.issue_id
WHERE c.issue_id = ANY (@issue_ids::uuid[])
  AND i.status = 'done'
  AND c.done_at = i.status_at
ORDER BY c.at, c.user_id;

-- name: InsertAnswer :exec
INSERT INTO issue_confirmations (issue_id, user_id, done_at, fixed, at)
VALUES (@issue_id, @user_id, @done_at, @fixed, @at)
ON CONFLICT DO NOTHING;

-- name: InsertEvent :exec
INSERT INTO issue_events (issue_id, kind, user_id, status, comment, at)
VALUES (@issue_id, @kind, NULLIF(@user_id::bigint, 0), @status, @comment, @at);
