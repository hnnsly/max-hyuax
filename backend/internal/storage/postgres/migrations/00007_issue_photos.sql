-- +goose Up

-- Фото к заявке: здесь только метаданные, сами файлы лежат в хранилище файлов (ADR-015).
-- Файлы перекодированы в JPEG без EXIF, поэтому геометки и данные камеры не хранятся.
CREATE TABLE issue_photos (
    id          uuid PRIMARY KEY,
    issue_id    uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    uploaded_by bigint NOT NULL REFERENCES users (id),
    width       integer NOT NULL,
    height      integer NOT NULL,
    size_bytes  integer NOT NULL,
    created_at  timestamptz NOT NULL
);

CREATE INDEX issue_photos_issue_idx ON issue_photos (issue_id, created_at);

-- +goose Down
DROP TABLE issue_photos;
