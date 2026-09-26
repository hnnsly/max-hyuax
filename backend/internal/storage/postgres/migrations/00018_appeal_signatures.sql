-- +goose Up

-- Подписи соседей под коллективным обращением в жилинспекцию по просроченной заявке (ADR-023).
-- ФИО и квартира необязательны: пустые строки — подпись идёт только в счётчик «поддержали».
CREATE TABLE appeal_signatures (
    issue_id   uuid        NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id    bigint      NOT NULL REFERENCES users (id),
    full_name  text        NOT NULL DEFAULT '' CHECK (char_length(full_name) <= 100),
    apartment  text        NOT NULL DEFAULT '' CHECK (char_length(apartment) <= 10),
    signed_at  timestamptz NOT NULL,
    PRIMARY KEY (issue_id, user_id)
);

CREATE INDEX appeal_signatures_user ON appeal_signatures (user_id);

-- +goose Down
DROP TABLE appeal_signatures;
