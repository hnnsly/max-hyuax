package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"dommax/internal/app"
	"dommax/internal/storage/postgres/sqlcdb"
)

type pendingRepo struct{ q *sqlcdb.Queries }

func (r pendingRepo) Set(ctx context.Context, userID int64, p app.BotPending) error {
	return r.q.SetBotPending(ctx, sqlcdb.SetBotPendingParams{
		UserID: userID, Action: p.Action, Ref: p.Ref, Text: p.Text, ExpiresAt: p.ExpiresAt,
	})
}

func (r pendingRepo) Take(ctx context.Context, userID int64, now time.Time) (app.BotPending, bool, error) {
	row, err := r.q.TakeBotPending(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.BotPending{}, false, nil
	}
	if err != nil {
		return app.BotPending{}, false, err
	}
	p := app.BotPending{Action: row.Action, Ref: row.Ref, Text: row.Text, ExpiresAt: row.ExpiresAt}
	return p, now.Before(p.ExpiresAt), nil
}
