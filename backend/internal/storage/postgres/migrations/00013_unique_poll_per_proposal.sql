-- +goose Up

-- По предложению открывается один опрос: повтор из второй вкладки или повторный запрос
-- ловит база, а не только интерфейс (ревью 25.09).
CREATE UNIQUE INDEX polls_proposal_idx ON polls (proposal_id) WHERE proposal_id IS NOT NULL;

-- +goose Down
DROP INDEX polls_proposal_idx;
