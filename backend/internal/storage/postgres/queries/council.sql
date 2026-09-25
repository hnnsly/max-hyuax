-- name: InsertProposal :exec
INSERT INTO proposals (id, house_id, author_id, text, status, answer, created_at)
VALUES (@id, @house_id, @author_id, @text, @status, @answer, @created_at);

-- name: GetProposal :one
SELECT * FROM proposals WHERE id = @id;

-- name: ReplyProposal :execrows
-- Ответ один: второй председатель (или повторное нажатие) не перезапишет первый.
UPDATE proposals SET status = @status, answer = @answer, answered_at = @answered_at
WHERE id = @id AND status = 'new';

-- name: ListHouseProposals :many
-- Папка председателя: новые первыми, дальше по дате.
SELECT * FROM proposals WHERE house_id = @house_id
ORDER BY (status = 'new') DESC, created_at DESC
LIMIT @max_rows;

-- name: ListAuthorProposals :many
SELECT * FROM proposals WHERE author_id = @author_id
ORDER BY created_at DESC
LIMIT @max_rows;

-- name: InsertPoll :exec
INSERT INTO polls (id, house_id, proposal_id, question, options, created_at, closes_at)
VALUES (@id, @house_id, @proposal_id, @question, @options, @created_at, @closes_at);

-- name: GetPoll :one
SELECT * FROM polls WHERE id = @id;

-- name: ListHousePolls :many
SELECT * FROM polls WHERE house_id = @house_id
ORDER BY created_at DESC
LIMIT @max_rows;

-- name: InsertVote :execrows
INSERT INTO poll_votes (poll_id, user_id, option) VALUES (@poll_id, @user_id, @option)
ON CONFLICT DO NOTHING;

-- name: PollTally :many
SELECT option, count(*)::int AS votes, bool_or(user_id = @user_id)::bool AS mine
FROM poll_votes WHERE poll_id = @poll_id
GROUP BY option;
