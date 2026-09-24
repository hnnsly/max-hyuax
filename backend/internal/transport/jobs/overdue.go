package jobs

import (
	"context"
	"log/slog"
	"time"
)

// OverdueInterval — как часто искать заявки с истёкшим сроком.
const OverdueInterval = 10 * time.Minute

// OverdueJob отмечает просроченные заявки: при старте и затем по интервалу.
// Уведомления попадают в outbox и уходят, когда бот включён.
type OverdueJob struct {
	mark  func(context.Context) (int, error)
	every time.Duration
	log   *slog.Logger
}

func NewOverdueJob(mark func(context.Context) (int, error), every time.Duration, log *slog.Logger) *OverdueJob {
	return &OverdueJob{mark: mark, every: every, log: log}
}

// Run работает, пока ctx не отменён.
func (j *OverdueJob) Run(ctx context.Context) error {
	t := time.NewTicker(j.every)
	defer t.Stop()
	for {
		switch n, err := j.mark(ctx); {
		case err != nil && ctx.Err() == nil:
			j.log.ErrorContext(ctx, "overdue check failed", "err", err)
		case n > 0:
			j.log.InfoContext(ctx, "issues marked overdue", "count", n)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}
