-- +goose Up

-- Плановые работы и отключения в доме (GEN_V4, ADR-024).
CREATE TABLE maintenance_alerts (
    id          uuid        PRIMARY KEY,
    house_id    text        NOT NULL REFERENCES houses (id) ON DELETE CASCADE,
    category    text        NOT NULL,
    title       text        NOT NULL CHECK (char_length(title) <= 200),
    description text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    starts_at   timestamptz NOT NULL,
    ends_at     timestamptz NOT NULL,
    created_by  bigint      NOT NULL REFERENCES users (id),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX maintenance_house_idx ON maintenance_alerts (house_id, ends_at DESC);

-- Запись жителей на личный приём в управляющую компанию (ПП РФ № 416 п. 28, ADR-024).
CREATE TABLE appointments (
    id         uuid        PRIMARY KEY,
    house_id   text        NOT NULL REFERENCES houses (id) ON DELETE CASCADE,
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    specialist text        NOT NULL,
    topic      text        NOT NULL CHECK (char_length(topic) <= 500),
    slot_at    timestamptz NOT NULL,
    status     text        NOT NULL DEFAULT 'booked' CHECK (status IN ('booked', 'cancelled', 'completed')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX appointments_user_idx ON appointments (user_id, slot_at DESC);
CREATE INDEX appointments_house_idx ON appointments (house_id, slot_at);

-- Демо-данные: активные плановые работы по отоплению в доме 17к2.
INSERT INTO maintenance_alerts (id, house_id, category, title, description, starts_at, ends_at, created_by, created_at)
SELECT '0190a000-0000-7000-8000-000000000401'::uuid,
       'h-17k2',
       'heating',
       'Опрессовка и регулировка стояков отопления',
       'Плановые работы на тепловом узле дома. Возможны кратковременные перебои с подачей горячей воды.',
       now() - interval '2 hours',
       now() + interval '18 hours',
       u.id,
       now() - interval '2 hours'
FROM users u WHERE u.demo_key = 'uk_operator_demo';

-- Демо-данные: запись жительницы Анны на приём к главному инженеру УК.
INSERT INTO appointments (id, house_id, user_id, specialist, topic, slot_at, status, created_at)
SELECT '0190a000-0000-7000-8000-000000000411'::uuid,
       'h-17k2',
       u.id,
       'chief_engineer',
       'Согласование замены радиатора отопления в квартире и перекрытия стояка',
       date_trunc('day', now() + interval '2 days') + interval '14 hours',
       'booked',
       now() - interval '1 day'
FROM users u WHERE u.demo_key = 'resident_demo_1';

-- +goose Down
DROP TABLE appointments;
DROP TABLE maintenance_alerts;
