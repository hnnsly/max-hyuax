-- +goose Up

-- Бот ждёт следующего сообщения пользователя: комментарий к «Не починили», предложение совету,
-- описание проблемы до выбора места. Одно действие на пользователя, живёт несколько минут.
CREATE TABLE bot_pending (
    user_id    bigint PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    action     text NOT NULL,
    ref        text NOT NULL DEFAULT '',
    text       text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE bot_pending;
