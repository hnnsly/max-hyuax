-- +goose Up

-- Ответы участников на «выполнено»: починили или нет. done_at — к какой отметке «выполнено»
-- относится ответ: после возврата в работу и нового «выполнено» жители отвечают заново.
CREATE TABLE issue_confirmations (
    issue_id uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id  bigint NOT NULL REFERENCES users (id),
    done_at  timestamptz NOT NULL,
    fixed    boolean NOT NULL,
    at       timestamptz NOT NULL,
    PRIMARY KEY (issue_id, user_id, done_at)
);

-- Когда жители в последний раз вернули заявку в работу: для группы «Вернули жители» и метрик.
ALTER TABLE issues ADD COLUMN reopened_at timestamptz;

-- Фото пользователя ищутся при удалении аккаунта.
CREATE INDEX issue_photos_uploader_idx ON issue_photos (uploaded_by);

-- +goose Down
DROP INDEX issue_photos_uploader_idx;
ALTER TABLE issues DROP COLUMN reopened_at;
DROP TABLE issue_confirmations;
