-- name: EnqueueNotification :exec
INSERT INTO outbox (kind, issue_id, user_id)
VALUES (@kind, @issue_id, @user_id)
ON CONFLICT (kind, issue_id, user_id) WHERE status = 'pending' DO NOTHING;

-- name: ClaimNotifications :many
-- Взятые строки переходят в sending и коммитятся сразу: новое изменение заявки
-- за время отправки попадёт в очередь отдельной строкой и не потеряется.
UPDATE outbox
SET status = 'sending', attempts = attempts + 1
WHERE id IN (
    SELECT o.id FROM outbox o
    WHERE o.status = 'pending' AND o.next_attempt_at <= now()
    ORDER BY o.id
    LIMIT @max_rows
    FOR UPDATE SKIP LOCKED
)
RETURNING id, kind, issue_id, user_id, attempts;

-- name: MarkNotificationDone :exec
UPDATE outbox SET status = 'done', last_error = '' WHERE id = @id;

-- name: RetryNotification :exec
-- Если за время отправки появилась новая строка для того же адресата, эта уже не нужна.
UPDATE outbox o
SET status = CASE
        WHEN @failed::bool THEN 'failed'
        WHEN EXISTS (
            SELECT 1 FROM outbox p
            WHERE p.status = 'pending' AND p.kind = o.kind AND p.issue_id = o.issue_id AND p.user_id = o.user_id
        ) THEN 'done'
        ELSE 'pending'
    END,
    next_attempt_at = @next_attempt_at,
    last_error = @last_error
WHERE o.id = @id;

-- name: ReleaseSending :exec
-- После перезапуска: недоотправленное возвращается в очередь, если его не заменила новая строка.
UPDATE outbox o
SET status = CASE
        WHEN EXISTS (
            SELECT 1 FROM outbox p
            WHERE p.status = 'pending' AND p.kind = o.kind AND p.issue_id = o.issue_id AND p.user_id = o.user_id
        ) THEN 'done'
        ELSE 'pending'
    END
WHERE o.status = 'sending';

-- name: GetCardMID :one
SELECT mid FROM bot_cards WHERE issue_id = @issue_id AND user_id = @user_id;

-- name: SaveCardMID :exec
INSERT INTO bot_cards (issue_id, user_id, mid)
VALUES (@issue_id, @user_id, @mid)
ON CONFLICT (issue_id, user_id) DO UPDATE SET mid = EXCLUDED.mid, updated_at = now();
