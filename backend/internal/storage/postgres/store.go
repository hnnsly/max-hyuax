// Пакет postgres — реализация портов app на PostgreSQL: pgx, запросы sqlc, миграции goose.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // драйвер database/sql для goose
	"github.com/pressly/goose/v3"

	"dommax/internal/app"
	"dommax/internal/storage/postgres/sqlcdb"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate накатывает недостающие миграции. Вызывается при старте api.
func Migrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	dir, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, dir)
	if err != nil {
		return err
	}
	defer provider.Close()
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Store реализует app.Store. Внутри InTx тот же тип работает поверх транзакции.
type Store struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
	tx   pgx.Tx // nil вне транзакции
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return &Store{pool: pool, q: sqlcdb.New(pool)}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) Issues() app.IssueRepo { return issueRepo{s} }
func (s *Store) Houses() app.HouseRepo { return houseRepo{s.q} }
func (s *Store) Users() app.UserRepo   { return userRepo{s.q} }

// InTx выполняет fn в транзакции; вложенный вызов переиспользует текущую.
func (s *Store) InTx(ctx context.Context, fn func(tx app.Store) error) error {
	if s.tx != nil {
		return fn(s)
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Store{pool: s.pool, q: s.q.WithTx(tx), tx: tx})
	})
}

// MarkUpdateProcessed запоминает ключ события MAX; false — событие уже обрабатывалось.
func (s *Store) MarkUpdateProcessed(ctx context.Context, key string) (bool, error) {
	n, err := s.q.MarkUpdateProcessed(ctx, key)
	return n == 1, err
}

// notFound переводит отсутствие строки в ошибку слоя приложения.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	return err
}
