-- +goose Up

-- Пример данных для метрик УК. Синтетические заявки помечены sample = true: экран метрик
-- подписывает их «Пример данных», а сервис сдвигает их даты вперёд, чтобы пример не устаревал.
ALTER TABLE issues ADD COLUMN sample boolean NOT NULL DEFAULT false;
UPDATE issues SET sample = true WHERE id::text LIKE '0190a000-%';

-- Синтетические жители нужны только для счётчиков участников: без имён, телефонов и MAX id.
INSERT INTO users (demo_key, role, consent_version, consent_at)
SELECT 'sample_' || n, 'resident', 'v1', now()
FROM generate_series(1, 12) AS n;

-- Все интервалы в часах от момента подачи: ответ УК, закрытие, срок.
CREATE TEMP TABLE sample_src (
    n          int PRIMARY KEY,
    house_id   text NOT NULL,
    object_id  text,
    category   text NOT NULL,
    title      text NOT NULL,
    created_h  numeric NOT NULL, -- сколько часов назад подана
    response_h numeric,          -- когда УК впервые сменила статус; NULL — ещё не ответила
    status     text NOT NULL,    -- статус сейчас
    closed_h   numeric,          -- когда закрыта; NULL — открыта
    deadline_h numeric NOT NULL,
    reporters  int NOT NULL,     -- автор и присоединившиеся
    comment    text NOT NULL
);

INSERT INTO sample_src VALUES
    (101, 'h-13k1', 'h-13k1-e1-lift',  'lift',     'Лифт не работает',            385, 3,  'done',        20,   24,  5, 'Заменили кнопку вызова'),
    (102, 'h-15',   'h-15-e2-light',   'lighting', 'Не горит свет на лестнице',   360, 9,  'done',        50,   72,  3, 'Заменили лампы на 4 и 5 этажах'),
    (103, 'h-17k1', 'h-17k1-roof',     'leak',     'Протечка с кровли',           340, 7,  'done',        30,   24,  6, 'Кровлю залатали, нужна плановая замена'),
    (104, 'h-19',   'h-19-trash',      'garbage',  'Засор мусоропровода',         320, 11, 'done',        60,   72,  4, 'Засор устранён'),
    (105, 'h-21k1', 'h-21k1-e3-lift',  'lift',     'Лифт застревает между этажами', 300, 6, 'done',       22,   24,  8, 'Отрегулировали двери кабины'),
    (106, 'h-17k2', 'h-17k2-e3-light', 'lighting', 'Не горит свет на лестнице',   280, 8,  'done',        90,   72,  2, 'Заменили проводку на площадке'),
    (107, 'h-15',   NULL,              'door',     'Не работает домофон',         265, 5,  'rejected',    30,   72,  3, 'Домофон обслуживает другая организация, передали ей заявку'),
    (108, 'h-13k1', 'h-13k1-e2-light', 'lighting', 'Мигает свет в подъезде',      250, 7,  'done',        40,   72,  2, 'Заменили светильник'),
    (109, 'h-21k1', 'h-21k1-roof',     'leak',     'Протечка с кровли',           230, 4,  'done',        20,   24,  7, 'Прочистили водосток'),
    (110, 'h-17k1', 'h-17k1-e1-lift',  'lift',     'Лифт не работает',            200, 10, 'done',        23,   24,  6, 'Заменили плату управления'),
    (111, 'h-19',   'h-19-e1-light',   'lighting', 'Не горит свет на лестнице',   165, 6,  'done',        48,   72,  3, 'Заменили лампы'),
    (112, 'h-15',   'h-15-trash',      'garbage',  'Засор мусоропровода',         150, 4,  'done',        30,   72,  5, 'Засор устранён'),
    (113, 'h-21k1', 'h-21k1-e1-light', 'lighting', 'Не горит свет у входа',       140, 7,  'done',        80,   72,  2, 'Заменили светильник над входом'),
    (114, 'h-13k1', 'h-13k1-trash',    'garbage',  'Мусор не вывозят',            120, 3,  'done',        26,   72,  4, 'Вывезли, график вывоза уточнили'),
    (115, 'h-17k2', 'h-17k2-roof',     'leak',     'Протечка с кровли',           110, 5,  'in_progress', NULL, 24,  9, 'Ждём подрядчика по кровле'),
    (116, 'h-15',   'h-15-e4-lift',    'lift',     'Лифт не работает',            96,  2,  'done',        10,   24,  6, 'Лифт запущен'),
    (117, 'h-19',   'h-19-e2-lift',    'lift',     'Лифт не открывает двери',     80,  6,  'accepted',    NULL, 24,  5, 'Заказали запчасть'),
    (118, 'h-17k1', NULL,              'heating',  'Нет горячей воды',            70,  4,  'done',        16,   24,  7, 'Промыли стояк'),
    (119, 'h-21k1', 'h-21k1-e2-light', 'lighting', 'Не горит свет на лестнице',   50,  5,  'in_progress', NULL, 72,  2, 'Электрик придёт завтра'),
    (120, 'h-13k1', 'h-13k1-e3-lift',  'lift',     'Лифт не работает',            20,  3,  'accepted',    NULL, 24,  4, 'Мастер выехал'),
    (121, 'h-15',   'h-15-e1-light',   'lighting', 'Не горит свет на лестнице',   8,   NULL, 'sent',      NULL, 72,  1, ''),
    (122, 'h-17k2', NULL,              'other',    'Сломана скамейка у подъезда', 3,   NULL, 'sent',      NULL, 240, 1, '');

INSERT INTO issues (id, house_id, object_id, category, title, description, responsible_org_id,
                    status, status_at, status_comment, created_by, created_at, deadline_at, sample)
SELECT ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid, s.house_id, s.object_id, s.category, s.title, '',
       'org-orekh', s.status,
       now() - s.created_h * interval '1 hour' + COALESCE(s.closed_h, s.response_h, 0) * interval '1 hour',
       s.comment,
       (SELECT id FROM users WHERE demo_key = 'sample_' || (s.n % 12 + 1)),
       now() - s.created_h * interval '1 hour',
       now() - s.created_h * interval '1 hour' + s.deadline_h * interval '1 hour',
       true
FROM sample_src s;

-- Автор (j = 0) и соседи присоединяются с интервалом 40 минут.
INSERT INTO issue_participants (issue_id, user_id, joined_at)
SELECT i.id, u.id, i.created_at + j * interval '40 minutes'
FROM sample_src s
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
JOIN sample_src s ON i.id = ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid
UNION ALL
-- Первый ответ УК: у закрытых заявок это «принята», у открытых — текущий статус.
SELECT i.id, 'status_changed', NULL,
       CASE WHEN s.closed_h IS NULL THEN s.status ELSE 'accepted' END,
       CASE WHEN s.closed_h IS NULL THEN s.comment ELSE '' END,
       i.created_at + s.response_h * interval '1 hour'
FROM sample_src s
JOIN issues i ON i.id = ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid
WHERE s.response_h IS NOT NULL
UNION ALL
SELECT i.id, 'status_changed', NULL, s.status, s.comment, i.created_at + s.closed_h * interval '1 hour'
FROM sample_src s
JOIN issues i ON i.id = ('0190a000-0000-7000-8000-' || lpad(s.n::text, 12, '0'))::uuid
WHERE s.closed_h IS NOT NULL;

DROP TABLE sample_src;

-- +goose Down
DELETE FROM issue_events WHERE issue_id::text LIKE '0190a000-0000-7000-8000-0000000001%';
DELETE FROM issue_participants WHERE issue_id::text LIKE '0190a000-0000-7000-8000-0000000001%';
DELETE FROM issues WHERE id::text LIKE '0190a000-0000-7000-8000-0000000001%';
DELETE FROM users WHERE demo_key LIKE 'sample\_%';
ALTER TABLE issues DROP COLUMN sample;
