-- +goose Up

-- Пример данных для проверки ремонта жителями (заявки с sample = true из 00002 и 00006).
-- Автор подтверждает ремонт через 3 часа после «выполнено»; по заявкам 106 и 113 ответа нет.
INSERT INTO issue_confirmations (issue_id, user_id, done_at, fixed, at)
SELECT i.id, i.created_by, i.status_at, true, i.status_at + interval '3 hours'
FROM issues i
WHERE i.sample AND i.status = 'done'
  AND i.id NOT IN ('0190a000-0000-7000-8000-000000000106', '0190a000-0000-7000-8000-000000000113');

INSERT INTO issue_events (issue_id, kind, user_id, status, comment, at)
SELECT c.issue_id, 'confirmed', c.user_id, 'done', '', c.at
FROM issue_confirmations c
JOIN issues i ON i.id = c.issue_id
WHERE i.sample;

-- Заявку 119 УК отметила выполненной через 20 часов, а через 26 часов жительница вернула её в работу:
-- срок заново по справочнику (свет: 3 рабочих дня, для примера 72 часа).
UPDATE issues
SET status = 'in_progress', status_comment = '',
    status_at   = created_at + interval '26 hours',
    reopened_at = created_at + interval '26 hours',
    deadline_at = created_at + interval '98 hours'
WHERE id = '0190a000-0000-7000-8000-000000000119';

INSERT INTO issue_events (issue_id, kind, user_id, status, comment, at)
SELECT id, 'status_changed', NULL, 'done', 'Заменили лампы', created_at + interval '20 hours'
FROM issues WHERE id = '0190a000-0000-7000-8000-000000000119'
UNION ALL
SELECT id, 'reopened', created_by, 'in_progress', 'На третьем этаже снова темно', created_at + interval '26 hours'
FROM issues WHERE id = '0190a000-0000-7000-8000-000000000119';

-- Ответ «не починили» относится к «выполнено» через 20 часов.
INSERT INTO issue_confirmations (issue_id, user_id, done_at, fixed, at)
SELECT id, created_by, created_at + interval '20 hours', false, created_at + interval '26 hours'
FROM issues WHERE id = '0190a000-0000-7000-8000-000000000119';

-- +goose Down
DELETE FROM issue_events WHERE issue_id::text LIKE '0190a000-0000-7000-8000-0000000001%' AND kind IN ('confirmed', 'reopened');
DELETE FROM issue_events WHERE issue_id = '0190a000-0000-7000-8000-000000000119' AND kind = 'status_changed' AND status = 'done';
DELETE FROM issue_confirmations WHERE issue_id::text LIKE '0190a000-0000-7000-8000-0000000001%';
UPDATE issues
SET status_at = created_at + interval '5 hours', status_comment = 'Электрик придёт завтра',
    reopened_at = NULL, deadline_at = created_at + interval '72 hours'
WHERE id = '0190a000-0000-7000-8000-000000000119';
