-- +goose Up

-- Совет дома (ADR-017): председатель — житель с отметкой дома, где он председатель.
ALTER TABLE users ADD COLUMN chairman_house_id text REFERENCES houses (id);

-- Предложения жителей председателю. Автор хранится для «Моих предложений», председателю не показывается.
CREATE TABLE proposals (
    id          uuid PRIMARY KEY,
    house_id    text NOT NULL REFERENCES houses (id),
    author_id   bigint NOT NULL REFERENCES users (id),
    text        text NOT NULL,
    status      text NOT NULL DEFAULT 'new',
    answer      text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL,
    answered_at timestamptz
);
CREATE INDEX proposals_house_idx ON proposals (house_id, created_at DESC);
CREATE INDEX proposals_author_idx ON proposals (author_id, created_at DESC);

-- Опросы без юридической силы: один голос на жителя.
CREATE TABLE polls (
    id          uuid PRIMARY KEY,
    house_id    text NOT NULL REFERENCES houses (id),
    proposal_id uuid REFERENCES proposals (id),
    question    text NOT NULL,
    options     text[] NOT NULL,
    created_at  timestamptz NOT NULL,
    closes_at   timestamptz NOT NULL
);
CREATE INDEX polls_house_idx ON polls (house_id, created_at DESC);

CREATE TABLE poll_votes (
    poll_id  uuid NOT NULL REFERENCES polls (id),
    user_id  bigint NOT NULL REFERENCES users (id),
    option   integer NOT NULL,
    voted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (poll_id, user_id)
);

-- Демо-председатель совета дома 17к2.
INSERT INTO users (demo_key, first_name, house_id, role, consent_version, consent_at, chairman_house_id)
VALUES ('chairman_demo', 'Нина', 'h-17k2', 'resident', 'v1', now(), 'h-17k2');

-- Пример: три предложения соседей, из одного председатель сделал опрос. Голоса синтетических жителей.
INSERT INTO proposals (id, house_id, author_id, text, status, answer, created_at, answered_at)
SELECT v.id::uuid, 'h-17k2', u.id, v.text, v.status, v.answer, now() - v.ago::interval,
       CASE WHEN v.status = 'new' THEN NULL ELSE now() - v.ago::interval + interval '1 day' END
FROM (VALUES
    ('0190a000-0000-7000-8000-000000000301', 'resident_demo_2', 'Поставить велопарковку у второго подъезда, велосипеды стоят на площадках', 'new', '', '1 day'),
    ('0190a000-0000-7000-8000-000000000302', 'resident_demo_1', 'Покрасить лавочки у детской площадки, краска облезла', 'accepted', 'Передала в УК, обещали покрасить до конца октября.', '6 days'),
    ('0190a000-0000-7000-8000-000000000303', 'resident_demo_2', 'Поставить шлагбаум на въезде во двор: чужие машины занимают места', 'accepted', 'Выношу на опрос, решим вместе.', '4 days')
) AS v(id, demo_key, text, status, answer, ago)
JOIN users u ON u.demo_key = v.demo_key;

INSERT INTO polls (id, house_id, proposal_id, question, options, created_at, closes_at) VALUES
    ('0190a000-0000-7000-8000-000000000311', 'h-17k2', '0190a000-0000-7000-8000-000000000303',
     'Ставим шлагбаум на въезде во двор?', ARRAY['За', 'Против', 'Воздержусь'],
     now() - interval '3 days', now() + interval '4 days');

INSERT INTO poll_votes (poll_id, user_id, option)
SELECT '0190a000-0000-7000-8000-000000000311', u.id, (ARRAY[0, 0, 1, 0, 2, 0, 1])[n]
FROM generate_series(1, 7) AS n
JOIN users u ON u.demo_key = 'sample_' || n;

-- +goose Down
DROP TABLE poll_votes;
DROP TABLE polls;
DROP TABLE proposals;
DELETE FROM users WHERE demo_key = 'chairman_demo';
ALTER TABLE users DROP COLUMN chairman_house_id;
