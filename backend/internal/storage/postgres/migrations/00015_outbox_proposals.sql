-- +goose Up

-- Очередь бота теперь несёт и уведомления совета дома: председателю о новом предложении,
-- автору об ответе. У строки ровно одна цель: заявка или предложение.
ALTER TABLE outbox ALTER COLUMN issue_id DROP NOT NULL;
ALTER TABLE outbox ADD COLUMN proposal_id uuid REFERENCES proposals (id) ON DELETE CASCADE;
ALTER TABLE outbox ADD CONSTRAINT outbox_one_target CHECK ((issue_id IS NULL) <> (proposal_id IS NULL));

CREATE INDEX users_chairman_idx ON users (chairman_house_id) WHERE chairman_house_id IS NOT NULL;

-- +goose Down
DROP INDEX users_chairman_idx;
DELETE FROM outbox WHERE proposal_id IS NOT NULL;
ALTER TABLE outbox DROP CONSTRAINT outbox_one_target;
ALTER TABLE outbox DROP COLUMN proposal_id;
ALTER TABLE outbox ALTER COLUMN issue_id SET NOT NULL;
