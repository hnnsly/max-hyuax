-- +goose Up

-- Организации: УК, ТСЖ, РСО и т.д. source = 'model' помечает модельные записи.
CREATE TABLE organizations (
    id               text PRIMARY KEY,
    type             text NOT NULL,
    name             text NOT NULL,
    phone_office     text NOT NULL DEFAULT '',
    phone_dispatcher text NOT NULL DEFAULT '',
    phone_emergency  text NOT NULL DEFAULT '',
    schedule         text NOT NULL DEFAULT '',
    source           text NOT NULL DEFAULT 'model'
);

CREATE TABLE houses (
    id              text PRIMARY KEY,
    address         text NOT NULL,
    district        text NOT NULL DEFAULT '',
    year_built      integer NOT NULL DEFAULT 0,
    floors          integer NOT NULL DEFAULT 0,
    entrances_count integer NOT NULL DEFAULT 0,
    organization_id text NOT NULL REFERENCES organizations (id),
    lat             double precision NOT NULL DEFAULT 0,
    lon             double precision NOT NULL DEFAULT 0,
    source          text NOT NULL DEFAULT 'model'
);

CREATE TABLE entrances (
    id       text PRIMARY KEY,
    house_id text NOT NULL REFERENCES houses (id),
    number   integer NOT NULL,
    UNIQUE (house_id, number)
);

-- Объект общего имущества с QR-кодом: qr_code попадает в диплинк startapp=o_<qr_code>.
CREATE TABLE asset_objects (
    id          text PRIMARY KEY,
    house_id    text NOT NULL REFERENCES houses (id),
    entrance_id text REFERENCES entrances (id),
    category    text NOT NULL,
    label       text NOT NULL,
    qr_code     text NOT NULL UNIQUE
);

CREATE TABLE users (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    max_user_id     bigint UNIQUE,
    demo_key        text UNIQUE,
    first_name      text NOT NULL DEFAULT '',
    phone           text NOT NULL DEFAULT '',
    house_id        text REFERENCES houses (id),
    role            text NOT NULL DEFAULT 'resident',
    organization_id text REFERENCES organizations (id),
    consent_version text NOT NULL DEFAULT '',
    consent_at      timestamptz,
    deleted_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- Сквозной номер заявки для людей («Заявка № 142»); id остаётся uuid.
CREATE SEQUENCE issue_number_seq START 101;

CREATE TABLE issues (
    id                 uuid PRIMARY KEY,
    number             bigint NOT NULL UNIQUE DEFAULT nextval('issue_number_seq'),
    house_id           text NOT NULL REFERENCES houses (id),
    object_id          text REFERENCES asset_objects (id),
    category           text NOT NULL,
    title              text NOT NULL,
    description        text NOT NULL DEFAULT '',
    responsible_org_id text NOT NULL REFERENCES organizations (id),
    status             text NOT NULL,
    status_at          timestamptz NOT NULL,
    status_comment     text NOT NULL DEFAULT '',
    created_by         bigint NOT NULL REFERENCES users (id),
    created_at         timestamptz NOT NULL,
    deadline_at        timestamptz NOT NULL
);

CREATE INDEX issues_house_idx ON issues (house_id, category, created_at DESC);
CREATE INDEX issues_org_open_idx ON issues (responsible_org_id, deadline_at)
    WHERE status NOT IN ('done', 'rejected');

CREATE TABLE issue_participants (
    issue_id  uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id   bigint NOT NULL REFERENCES users (id),
    joined_at timestamptz NOT NULL,
    PRIMARY KEY (issue_id, user_id)
);

-- Журнал доменных событий заявки: история для карточки и основание для outbox.
CREATE TABLE issue_events (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    issue_id uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    kind     text NOT NULL,
    user_id  bigint REFERENCES users (id),
    status   text NOT NULL,
    comment  text NOT NULL DEFAULT '',
    at       timestamptz NOT NULL
);

CREATE INDEX issue_events_issue_idx ON issue_events (issue_id, at);

-- Webhook MAX может прислать событие повторно: ключ обработанного события хранится здесь.
CREATE TABLE processed_updates (
    key         text PRIMARY KEY,
    received_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_updates;
DROP TABLE issue_events;
DROP TABLE issue_participants;
DROP TABLE issues;
DROP SEQUENCE issue_number_seq;
DROP TABLE users;
DROP TABLE asset_objects;
DROP TABLE entrances;
DROP TABLE houses;
DROP TABLE organizations;
