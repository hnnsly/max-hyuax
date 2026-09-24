-- +goose Up

-- Когда отмечена просрочка: участники получают уведомление о ней ровно один раз.
ALTER TABLE issues ADD COLUMN overdue_at timestamptz;

CREATE INDEX issues_overdue_scan_idx ON issues (deadline_at)
    WHERE overdue_at IS NULL AND status NOT IN ('done', 'rejected');

-- +goose Down
DROP INDEX issues_overdue_scan_idx;
ALTER TABLE issues DROP COLUMN overdue_at;
