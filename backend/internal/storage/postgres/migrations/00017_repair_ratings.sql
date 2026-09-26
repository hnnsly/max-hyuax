-- +goose Up

-- Оценка ремонта 1–5 после «Починили» (ADR-022). Хранится в самом ответе: одна оценка на
-- подтверждённый ремонт, NULL — житель не оценивал.
ALTER TABLE issue_confirmations
    ADD COLUMN stars smallint CHECK (stars BETWEEN 1 AND 5);

-- Пример данных: жители Ясеневого двора довольны, Орехового в целом тоже, Каширский квартал хуже.
-- Разброс по заявке и жителю, чтобы средние не выглядели круглыми.
UPDATE issue_confirmations c
SET stars = CASE i.responsible_org_id
                WHEN 'org-yasen' THEN 5 - abs(hashtext(c.issue_id::text || c.user_id::text)) % 2
                WHEN 'org-orekh' THEN 5 - abs(hashtext(c.issue_id::text || c.user_id::text)) % 3
                ELSE 3 - abs(hashtext(c.issue_id::text || c.user_id::text)) % 2
            END
FROM issues i
WHERE i.id = c.issue_id AND i.sample AND c.fixed;

-- +goose Down
ALTER TABLE issue_confirmations DROP COLUMN stars;
