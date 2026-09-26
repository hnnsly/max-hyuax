-- name: SignAppeal :exec
-- Повторная подпись обновляет ФИО и квартиру: житель мог сначала подписать без них.
INSERT INTO appeal_signatures (issue_id, user_id, full_name, apartment, signed_at)
VALUES (@issue_id, @user_id, @full_name, @apartment, @signed_at)
ON CONFLICT (issue_id, user_id) DO UPDATE SET
    full_name = EXCLUDED.full_name, apartment = EXCLUDED.apartment, signed_at = EXCLUDED.signed_at;

-- name: WithdrawAppeal :exec
DELETE FROM appeal_signatures WHERE issue_id = @issue_id AND user_id = @user_id;

-- name: ListAppealSignatures :many
SELECT issue_id::text AS issue_id, user_id, full_name, apartment, signed_at
FROM appeal_signatures
WHERE issue_id = @issue_id
ORDER BY signed_at, user_id;

-- name: ForgetAppealSignatures :exec
DELETE FROM appeal_signatures WHERE user_id = @user_id;
