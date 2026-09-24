-- +goose Up

-- Кабинет района (управа или жилинспекция): сравнение УК района, только чтение (ADR-017).
ALTER TABLE users ADD COLUMN district text NOT NULL DEFAULT '';

INSERT INTO users (demo_key, first_name, role, district, consent_version, consent_at)
VALUES ('district_demo', 'Управа района Зябликово', 'district', 'Зябликово', 'v1', now());

-- Ещё две УК района, чтобы было что сравнивать. Организации вымышленные, улицы реальные,
-- номера домов, годы постройки, телефоны и координаты условные (source = 'model').
INSERT INTO organizations (id, type, name, phone_office, phone_dispatcher, phone_emergency, schedule) VALUES
    ('org-yasen',  'uk', 'УК «Ясеневый двор»',     '+7 495 000-27-01', '+7 495 000-27-02', '+7 495 000-27-03',
     'Пн-Пт 8:00-17:00; диспетчерская круглосуточно'),
    ('org-kashir', 'uk', 'УК «Каширский квартал»', '+7 495 000-37-01', '+7 495 000-37-02', '+7 495 000-37-03',
     'Пн-Пт 9:00-18:00; диспетчерская круглосуточно');

INSERT INTO houses (id, address, district, year_built, floors, entrances_count, organization_id, lat, lon) VALUES
    ('h-yas32k1', 'Ясеневая улица, 32к1',       'Зябликово', 1982, 12, 3, 'org-yasen',  55.61900, 37.74400),
    ('h-yas34',   'Ясеневая улица, 34',         'Зябликово', 1985, 16, 2, 'org-yasen',  55.61960, 37.74650),
    ('h-vor22k1', 'Воронежская улица, 22к1',    'Зябликово', 1980, 9,  4, 'org-kashir', 55.60820, 37.76010),
    ('h-mdj8k2',  'улица Мусы Джалиля, 8к2',    'Зябликово', 1987, 14, 2, 'org-kashir', 55.62050, 37.74120);

INSERT INTO entrances (id, house_id, number)
SELECT h.id || '-e' || n, h.id, n
FROM houses h, generate_series(1, h.entrances_count) AS n
WHERE h.id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2');

INSERT INTO asset_objects (id, house_id, entrance_id, category, label, qr_code)
SELECT e.id || '-lift', e.house_id, e.id, 'lift', 'подъезд ' || e.number || ', пассажирский лифт', e.id || '-lift'
FROM entrances e WHERE e.house_id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2')
UNION ALL
SELECT e.id || '-light', e.house_id, e.id, 'lighting', 'подъезд ' || e.number || ', лестничная клетка', e.id || '-light'
FROM entrances e WHERE e.house_id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2')
UNION ALL
SELECT h.id || '-roof', h.id, NULL, 'leak', 'кровля', h.id || '-roof'
FROM houses h WHERE h.id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2')
UNION ALL
SELECT h.id || '-trash', h.id, NULL, 'garbage', 'мусоропровод', h.id || '-trash'
FROM houses h WHERE h.id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2');

-- Синтетические заявки по той же схеме, что в 00006: «Ясеневый двор» отвечает быстро и в срок,
-- у «Каширского квартала» долгие ответы и три открытые просроченные заявки.
CREATE TEMP TABLE district_src (
    n          int PRIMARY KEY,
    house_id   text NOT NULL,
    object_id  text,
    category   text NOT NULL,
    title      text NOT NULL,
    created_h  numeric NOT NULL,
    response_h numeric,
    status     text NOT NULL,
    closed_h   numeric,
    deadline_h numeric NOT NULL,
    reporters  int NOT NULL,
    comment    text NOT NULL
);

INSERT INTO district_src VALUES
    (201, 'h-yas32k1', 'h-yas32k1-e1-lift',  'lift',     'Лифт не работает',              300, 1,  'done',        12,  24, 4, 'Заменили датчик двери'),
    (202, 'h-yas34',   'h-yas34-e2-light',   'lighting', 'Не горит свет на лестнице',     240, 2,  'done',        20,  72, 2, 'Заменили лампы'),
    (203, 'h-yas32k1', 'h-yas32k1-trash',    'garbage',  'Засор мусоропровода',           180, 1,  'done',        8,   72, 3, 'Засор устранён'),
    (204, 'h-yas34',   'h-yas34-roof',       'leak',     'Протечка с кровли',             120, 2,  'done',        20,  24, 5, 'Залатали кровлю над вторым подъездом'),
    (205, 'h-yas32k1', 'h-yas32k1-e3-light', 'lighting', 'Мигает свет в подъезде',        60,  1,  'done',        6,   72, 2, 'Заменили светильник'),
    (206, 'h-yas34',   'h-yas34-e1-lift',    'lift',     'Лифт застревает между этажами', 20,  1,  'in_progress', NULL, 24, 3, 'Мастер на месте'),
    (207, 'h-yas32k1', NULL,                 'door',     'Не закрывается дверь подъезда', 5,   NULL, 'sent',      NULL, 72, 1, ''),
    (211, 'h-vor22k1', 'h-vor22k1-e2-lift',  'lift',     'Лифт не работает',              330, 30, 'done',        60,  24, 7, 'Заменили трос'),
    (212, 'h-mdj8k2',  'h-mdj8k2-roof',      'leak',     'Протечка с кровли',             280, 40, 'done',        120, 24, 6, 'Кровлю отремонтировали'),
    (213, 'h-vor22k1', 'h-vor22k1-e1-light', 'lighting', 'Не горит свет на лестнице',     200, 50, 'done',        100, 72, 3, 'Заменили лампы'),
    (214, 'h-mdj8k2',  'h-mdj8k2-trash',     'garbage',  'Мусор не вывозят',              150, NULL, 'sent',      NULL, 72, 5, ''),
    (215, 'h-vor22k1', 'h-vor22k1-e3-lift',  'lift',     'Лифт не открывает двери',       100, 48, 'accepted',    NULL, 24, 4, 'Заказали запчасть'),
    (216, 'h-mdj8k2',  NULL,                 'heating',  'Холодные батареи',              60,  NULL, 'sent',      NULL, 24, 6, ''),
    (217, 'h-vor22k1', 'h-vor22k1-e4-light', 'lighting', 'Не горит свет у входа',         30,  20, 'in_progress', NULL, 72, 2, 'Электрик придёт завтра');

INSERT INTO issues (id, house_id, object_id, category, title, description, responsible_org_id,
                    status, status_at, status_comment, created_by, created_at, deadline_at, sample)
SELECT ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid, s.house_id, s.object_id, s.category, s.title, '',
       h.organization_id, s.status,
       now() - s.created_h * interval '1 hour' + COALESCE(s.closed_h, s.response_h, 0) * interval '1 hour',
       s.comment,
       (SELECT id FROM users WHERE demo_key = 'sample_' || (s.n % 12 + 1)),
       now() - s.created_h * interval '1 hour',
       now() - s.created_h * interval '1 hour' + s.deadline_h * interval '1 hour',
       true
FROM district_src s
JOIN houses h ON h.id = s.house_id;

INSERT INTO issue_participants (issue_id, user_id, joined_at)
SELECT i.id, u.id, i.created_at + j * interval '40 minutes'
FROM district_src s
JOIN issues i ON i.id = ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid
CROSS JOIN generate_series(0, s.reporters - 1) AS j
JOIN users u ON u.demo_key = 'sample_' || ((s.n + j) % 12 + 1);

INSERT INTO issue_events (issue_id, kind, user_id, status, comment, at)
SELECT p.issue_id,
       CASE WHEN p.user_id = i.created_by THEN 'created' ELSE 'joined' END,
       p.user_id,
       CASE WHEN s.response_h IS NOT NULL AND p.joined_at >= i.created_at + s.response_h * interval '1 hour'
            THEN CASE WHEN s.closed_h IS NULL THEN s.status ELSE 'accepted' END
            ELSE 'sent' END,
       '', p.joined_at
FROM issue_participants p
JOIN issues i ON i.id = p.issue_id
JOIN district_src s ON i.id = ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid
UNION ALL
SELECT i.id, 'status_changed', NULL,
       CASE WHEN s.closed_h IS NULL THEN s.status ELSE 'accepted' END,
       CASE WHEN s.closed_h IS NULL THEN s.comment ELSE '' END,
       i.created_at + s.response_h * interval '1 hour'
FROM district_src s
JOIN issues i ON i.id = ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid
WHERE s.response_h IS NOT NULL
UNION ALL
SELECT i.id, 'status_changed', NULL, s.status, s.comment, i.created_at + s.closed_h * interval '1 hour'
FROM district_src s
JOIN issues i ON i.id = ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid
WHERE s.closed_h IS NOT NULL;

-- Проверка ремонта: в «Ясеневом дворе» жители подтвердили все ремонты, в «Каширском квартале» один.
INSERT INTO issue_confirmations (issue_id, user_id, done_at, fixed, at)
SELECT i.id, i.created_by, i.status_at, true, i.status_at + interval '2 hours'
FROM issues i
WHERE i.id IN ('0190a000-0000-7000-8000-000000000201', '0190a000-0000-7000-8000-000000000202',
               '0190a000-0000-7000-8000-000000000203', '0190a000-0000-7000-8000-000000000204',
               '0190a000-0000-7000-8000-000000000205', '0190a000-0000-7000-8000-000000000211');

INSERT INTO issue_events (issue_id, kind, user_id, status, comment, at)
SELECT c.issue_id, 'confirmed', c.user_id, 'done', '', c.at
FROM issue_confirmations c
WHERE c.issue_id::text LIKE '0190a000-0000-7000-8000-0000000002%';

DROP TABLE district_src;

-- +goose Down
DELETE FROM issue_events WHERE issue_id::text LIKE '0190a000-0000-7000-8000-0000000002%';
DELETE FROM issue_confirmations WHERE issue_id::text LIKE '0190a000-0000-7000-8000-0000000002%';
DELETE FROM issue_participants WHERE issue_id::text LIKE '0190a000-0000-7000-8000-0000000002%';
DELETE FROM issues WHERE id::text LIKE '0190a000-0000-7000-8000-0000000002%';
DELETE FROM asset_objects WHERE house_id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2');
DELETE FROM entrances WHERE house_id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2');
DELETE FROM houses WHERE id IN ('h-yas32k1', 'h-yas34', 'h-vor22k1', 'h-mdj8k2');
DELETE FROM organizations WHERE id IN ('org-yasen', 'org-kashir');
DELETE FROM users WHERE demo_key = 'district_demo';
ALTER TABLE users DROP COLUMN district;
