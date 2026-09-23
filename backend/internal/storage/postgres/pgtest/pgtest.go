// Пакет pgtest создаёт для интеграционных тестов отдельную временную базу с миграциями.
package pgtest

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5"

	"dommax/internal/storage/postgres"
)

// Fresh создаёт базу dommax_test_<случайный суффикс> на сервере из DATABASE_URL,
// накатывает миграции (включая демо-данные) и возвращает её DSN и функцию удаления.
func Fresh(ctx context.Context) (dsn string, drop func(), err error) {
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		return "", nil, fmt.Errorf("DATABASE_URL is not set: run task db and use task test:integration")
	}
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		return "", nil, err
	}
	name := "dommax_test_" + rand.Text()[:10]
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		admin.Close(ctx)
		return "", nil, err
	}
	u, err := url.Parse(base)
	if err != nil {
		admin.Close(ctx)
		return "", nil, err
	}
	u.Path = "/" + name
	dsn = u.String()
	drop = func() {
		// WITH (FORCE) обрывает соединения пула, если тест их не закрыл.
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		admin.Close(context.Background())
	}
	if err := postgres.Migrate(ctx, dsn); err != nil {
		drop()
		return "", nil, err
	}
	return dsn, drop, nil
}
