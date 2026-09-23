-- +goose Up

-- Модельные данные для демонстрации (source = 'model'). Улица реальная, УК, телефоны,
-- годы постройки и координаты условные. Заявки и пользователи синтетические.

INSERT INTO organizations (id, type, name, phone_office, phone_dispatcher, phone_emergency, schedule) VALUES
    ('org-orekh', 'uk', 'УК «Ореховый квартал»', '+7 495 000-17-01', '+7 495 000-17-02', '+7 495 000-17-03',
     'Пн-Чт 8:00-17:00, Пт 8:00-15:45; диспетчерская круглосуточно');

INSERT INTO houses (id, address, district, year_built, floors, entrances_count, organization_id, lat, lon) VALUES
    ('h-13k1', 'Ореховый бульвар, 13к1', 'Зябликово', 1981, 12, 3, 'org-orekh', 55.61410, 37.74010),
    ('h-15',   'Ореховый бульвар, 15',   'Зябликово', 1979, 9,  4, 'org-orekh', 55.61350, 37.74250),
    ('h-17k1', 'Ореховый бульвар, 17к1', 'Зябликово', 1983, 12, 2, 'org-orekh', 55.61290, 37.74480),
    ('h-17k2', 'Ореховый бульвар, 17к2', 'Зябликово', 1984, 12, 3, 'org-orekh', 55.61240, 37.74620),
    ('h-19',   'Ореховый бульвар, 19',   'Зябликово', 1986, 16, 2, 'org-orekh', 55.61180, 37.74850),
    ('h-21k1', 'Ореховый бульвар, 21к1', 'Зябликово', 1988, 14, 4, 'org-orekh', 55.61120, 37.75070);

INSERT INTO entrances (id, house_id, number)
SELECT h.id || '-e' || n, h.id, n
FROM houses h, generate_series(1, h.entrances_count) AS n;

-- Объекты с QR-кодами: лифт и свет в каждом подъезде, плюс кровля и мусоропровод дома.
INSERT INTO asset_objects (id, house_id, entrance_id, category, label, qr_code)
SELECT e.id || '-lift', e.house_id, e.id, 'lift', 'подъезд ' || e.number || ', пассажирский лифт', e.id || '-lift'
FROM entrances e
UNION ALL
SELECT e.id || '-light', e.house_id, e.id, 'lighting', 'подъезд ' || e.number || ', лестничная клетка', e.id || '-light'
FROM entrances e
UNION ALL
SELECT h.id || '-roof', h.id, NULL, 'leak', 'кровля', h.id || '-roof'
FROM houses h
UNION ALL
SELECT h.id || '-trash', h.id, NULL, 'garbage', 'мусоропровод', h.id || '-trash'
FROM houses h;

-- Демо-пользователи для проверяющих (вход через POST /api/v1/auth/demo).
INSERT INTO users (demo_key, first_name, house_id, role, organization_id, consent_version, consent_at) VALUES
    ('resident_demo_1',  'Анна',        'h-17k2', 'resident',    NULL,        'v1', now()),
    ('resident_demo_2',  'Сергей',      'h-17k2', 'resident',    NULL,        'v1', now()),
    ('uk_operator_demo', 'Оператор УК', NULL,     'uk_operator', 'org-orekh', 'v1', now());

-- Синтетические заявки в разных статусах.
INSERT INTO issues (id, house_id, object_id, category, title, description, responsible_org_id,
                    status, status_at, status_comment, created_by, created_at, deadline_at)
SELECT v.id::uuid, 'h-17k2', v.object_id, v.category, v.title, v.description, 'org-orekh',
       v.status, now() - v.status_ago, v.comment,
       (SELECT id FROM users WHERE demo_key = 'resident_demo_2'),
       now() - v.created_ago, now() - v.created_ago + v.deadline_in
FROM (VALUES
    ('0190a000-0000-7000-8000-000000000001', 'h-17k2-e2-lift', 'lift', 'Лифт не работает, подъезд 2',
     'Кабина не приходит на вызов, горит индикатор', 'in_progress', interval '3 hours',
     'Мастер приедет сегодня до 18:00', interval '1 day', interval '2 days'),
    ('0190a000-0000-7000-8000-000000000002', 'h-17k2-e1-light', 'lighting', 'Не горит свет на лестнице, подъезд 1',
     'Темно между 3 и 5 этажами', 'sent', interval '5 hours',
     '', interval '5 hours', interval '3 days'),
    ('0190a000-0000-7000-8000-000000000003', 'h-17k2-trash', 'garbage', 'Засор мусоропровода',
     'Мусор не проходит, запах на всех этажах', 'done', interval '2 days',
     'Засор устранён', interval '4 days', interval '3 days')
) AS v(id, object_id, category, title, description, status, status_ago, comment, created_ago, deadline_in);

INSERT INTO issue_participants (issue_id, user_id, joined_at)
SELECT i.id, i.created_by, i.created_at FROM issues i;

INSERT INTO issue_events (issue_id, kind, user_id, status, at)
SELECT i.id, 'created', i.created_by, 'sent', i.created_at FROM issues i;

-- +goose Down
DELETE FROM issue_events;
DELETE FROM issue_participants;
DELETE FROM issues;
DELETE FROM users WHERE demo_key IS NOT NULL;
DELETE FROM asset_objects;
DELETE FROM entrances;
DELETE FROM houses;
DELETE FROM organizations;
