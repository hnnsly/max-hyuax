-- +goose Up

-- Сообщение с карточкой заявки у каждого участника: его редактируем при изменениях.
CREATE TABLE bot_cards (
    issue_id   uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users (id),
    mid        text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, user_id)
);

-- Очередь исходящих сообщений бота. Хранится только адресат: текст собирается при отправке
-- из актуальной заявки. status: pending → sending → done | failed.
CREATE TABLE outbox (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind            text NOT NULL,
    issue_id        uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id         bigint NOT NULL REFERENCES users (id),
    status          text NOT NULL DEFAULT 'pending',
    attempts        integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- Серия изменений одной заявки до отправки схлопывается в одно уведомление адресату.
CREATE UNIQUE INDEX outbox_pending_uniq ON outbox (kind, issue_id, user_id) WHERE status = 'pending';
CREATE INDEX outbox_due_idx ON outbox (next_attempt_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE outbox;
DROP TABLE bot_cards;
