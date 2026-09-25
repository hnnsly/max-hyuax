-- +goose Up

-- Роль, взятая через /role на демо-стенде: такой «сотрудник УК» не получает имён и телефонов
-- настоящих жителей, только синтетические данные демо-пользователей.
ALTER TABLE users ADD COLUMN role_switched boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN role_switched;
