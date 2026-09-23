-- +goose Up

-- Демо-заявки: заголовок без места (место показывается строкой объекта) и история статусов,
-- чтобы хронология в карточке совпадала со статусом. Затрагивает только синтетические заявки.
UPDATE issues SET title = 'Лифт не работает' WHERE id = '0190a000-0000-7000-8000-000000000001';
UPDATE issues SET title = 'Не горит свет на лестнице' WHERE id = '0190a000-0000-7000-8000-000000000002';
UPDATE issues SET title = 'Засор мусоропровода' WHERE id = '0190a000-0000-7000-8000-000000000003';

INSERT INTO issue_events (issue_id, kind, user_id, status, comment, at)
SELECT i.id, 'status_changed', NULL, i.status, i.status_comment, i.status_at
FROM issues i
WHERE i.id IN ('0190a000-0000-7000-8000-000000000001', '0190a000-0000-7000-8000-000000000003');

-- +goose Down
DELETE FROM issue_events
WHERE kind = 'status_changed'
  AND issue_id IN ('0190a000-0000-7000-8000-000000000001', '0190a000-0000-7000-8000-000000000003');
