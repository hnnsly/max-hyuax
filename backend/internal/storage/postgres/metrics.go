package postgres

import (
	"context"
	"time"

	"dommax/internal/app"
	"dommax/internal/storage/postgres/sqlcdb"
)

func (r issueRepo) FirstResponses(ctx context.Context, orgID string, since time.Time) ([]app.FirstResponse, error) {
	rows, err := r.s.q.ListFirstResponses(ctx, sqlcdb.ListFirstResponsesParams{OrgID: orgID, Since: since})
	return mapSlice(rows, func(row sqlcdb.ListFirstResponsesRow) app.FirstResponse {
		return app.FirstResponse{CreatedAt: row.CreatedAt, RespondedAt: row.RespondedAt}
	}), err
}

func (r issueRepo) OrgCounts(ctx context.Context, orgID string, since, now time.Time) (app.OrgCounts, error) {
	row, err := r.s.q.OrgMetricCounts(ctx, sqlcdb.OrgMetricCountsParams{OrgID: orgID, Since: since, Now: now})
	return app.OrgCounts{
		Issues: int(row.Issues), Reports: int(row.Reports),
		ClosedTotal: int(row.ClosedTotal), ClosedOnTime: int(row.ClosedOnTime),
		OpenTotal: int(row.OpenTotal), OverdueOpen: int(row.OverdueOpen),
		SampleData: row.SampleData,
	}, err
}

// ShiftSampleData сдвигает даты примера данных (issues.sample) вперёд на целое число дней,
// чтобы его последнее событие было не старше суток: метрики на демо-стенде не пустеют.
// Возвращает, на сколько дней сдвинуто. Расчёт и сдвиг идут в одной транзакции под блокировкой:
// параллельный запуск дождётся первого и увидит уже свежие даты.
func (s *Store) ShiftSampleData(ctx context.Context, now time.Time) (int, error) {
	days := 0
	err := s.InTx(ctx, func(tx app.Store) error {
		q := tx.(*Store).q
		if err := q.LockSampleShift(ctx); err != nil {
			return err
		}
		latest, err := q.SampleLatestAt(ctx, now)
		if err != nil {
			return err
		}
		if days = int(now.Sub(latest) / (24 * time.Hour)); days < 1 {
			days = 0
			return nil
		}
		for _, shift := range []func(context.Context, int32) error{q.ShiftSampleIssues, q.ShiftSampleEvents, q.ShiftSampleParticipants} {
			if err := shift(ctx, int32(days)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return days, nil
}
