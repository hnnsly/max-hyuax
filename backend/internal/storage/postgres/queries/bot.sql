-- name: SetBotPending :exec
INSERT INTO bot_pending (user_id, action, ref, text, expires_at)
VALUES (@user_id, @action, @ref, @text, @expires_at)
ON CONFLICT (user_id) DO UPDATE SET
    action = EXCLUDED.action, ref = EXCLUDED.ref, text = EXCLUDED.text, expires_at = EXCLUDED.expires_at;

-- name: TakeBotPending :one
-- Забираем и стираем сразу: истёкшее действие тоже удаляется, срок проверяет репозиторий.
DELETE FROM bot_pending WHERE user_id = @user_id
RETURNING action, ref, text, expires_at;
