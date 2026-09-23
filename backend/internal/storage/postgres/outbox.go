package postgres

import (
	"context"
	"time"

	"dommax/internal/app"
	"dommax/internal/storage/postgres/sqlcdb"
)

type outboxRepo struct{ q *sqlcdb.Queries }

func (s *Store) Outbox() app.OutboxRepo { return outboxRepo{s.q} }

func (r outboxRepo) Enqueue(ctx context.Context, notes []app.Notification) error {
	for _, n := range notes {
		err := r.q.EnqueueNotification(ctx, sqlcdb.EnqueueNotificationParams{Kind: string(n.Kind), IssueID: n.IssueID, UserID: n.UserID})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r outboxRepo) Claim(ctx context.Context, n int) ([]app.OutboxItem, error) {
	rows, err := r.q.ClaimNotifications(ctx, int32(n))
	return mapSlice(rows, func(row sqlcdb.ClaimNotificationsRow) app.OutboxItem {
		return app.OutboxItem{
			ID:           row.ID,
			Notification: app.Notification{Kind: app.NotificationKind(row.Kind), IssueID: row.IssueID, UserID: row.UserID},
			Attempts:     int(row.Attempts),
		}
	}), err
}

func (r outboxRepo) Done(ctx context.Context, id int64) error {
	return r.q.MarkNotificationDone(ctx, id)
}

func (r outboxRepo) Retry(ctx context.Context, id int64, at time.Time, reason string, failed bool) error {
	return r.q.RetryNotification(ctx, sqlcdb.RetryNotificationParams{ID: id, NextAttemptAt: at, LastError: reason, Failed: failed})
}

func (r outboxRepo) Release(ctx context.Context) error { return r.q.ReleaseSending(ctx) }

func (r outboxRepo) CardMID(ctx context.Context, issueID string, userID int64) (string, error) {
	mid, err := r.q.GetCardMID(ctx, sqlcdb.GetCardMIDParams{IssueID: issueID, UserID: userID})
	return mid, notFound(err)
}

func (r outboxRepo) SaveCardMID(ctx context.Context, issueID string, userID int64, mid string) error {
	return r.q.SaveCardMID(ctx, sqlcdb.SaveCardMIDParams{IssueID: issueID, UserID: userID, Mid: mid})
}
