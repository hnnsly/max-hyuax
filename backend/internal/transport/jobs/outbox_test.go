package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/transport/jobs"
)

type clock struct {
	now   time.Time
	slept []time.Duration
}

func (c *clock) Now() time.Time { return c.now }
func (c *clock) Sleep(_ context.Context, d time.Duration) error {
	c.slept = append(c.slept, d)
	c.now = c.now.Add(d)
	return nil
}

func TestLimiterKeepsGlobalAndPerChatPace(t *testing.T) {
	c := &clock{now: time.Unix(0, 0)}
	l := jobs.NewLimiter(100*time.Millisecond, 500*time.Millisecond, c.Now, c.Sleep)
	for _, key := range []int64{1, 2, 1} {
		if err := l.Wait(t.Context(), key); err != nil {
			t.Fatal(err)
		}
	}
	// Первый — сразу; второй другой чат — через глобальный интервал; третий тот же чат — через 500 мс от первого.
	want := []time.Duration{100 * time.Millisecond, 400 * time.Millisecond}
	if len(c.slept) != 2 || c.slept[0] != want[0] || c.slept[1] != want[1] {
		t.Fatalf("slept = %v, want %v", c.slept, want)
	}
}

type queue struct {
	items   []app.OutboxItem
	done    []int64
	retried map[int64]bool // id → failed
}

func (q *queue) Claim(context.Context, int) ([]app.OutboxItem, error) {
	items := q.items
	q.items = nil
	return items, nil
}
func (q *queue) Done(_ context.Context, id int64) error { q.done = append(q.done, id); return nil }
func (q *queue) Release(context.Context) error          { return nil }
func (q *queue) Retry(_ context.Context, id int64, _ time.Time, _ string, failed bool) error {
	q.retried[id] = failed
	return nil
}

type limited struct{}

func (limited) Error() string     { return "429" }
func (limited) RateLimited() bool { return true }

func TestWorkerMarksDoneRetriesAndGivesUp(t *testing.T) {
	q := &queue{retried: map[int64]bool{}, items: []app.OutboxItem{
		{ID: 1, Notification: app.Notification{UserID: 10}, Attempts: 1},
		{ID: 2, Notification: app.Notification{UserID: 20}, Attempts: 1},
		{ID: 3, Notification: app.Notification{UserID: 30}, Attempts: 8},
	}}
	deliver := func(_ context.Context, n app.Notification) error {
		if n.UserID == 10 {
			return nil
		}
		return errors.New("boom")
	}
	c := &clock{now: time.Unix(0, 0)}
	w := jobs.NewOutboxWorker(q, deliver, jobs.NewLimiter(0, 0, c.Now, c.Sleep), slog.New(slog.NewTextHandler(io.Discard, nil)))
	n, err := w.RunOnce(t.Context())
	if err != nil || n != 3 {
		t.Fatalf("RunOnce = %d, %v", n, err)
	}
	if len(q.done) != 1 || q.done[0] != 1 {
		t.Fatalf("done = %v", q.done)
	}
	if failed, ok := q.retried[2]; !ok || failed {
		t.Fatalf("id 2 must be retried, got %v", q.retried)
	}
	if !q.retried[3] {
		t.Fatalf("id 3 exceeded attempts and must be failed, got %v", q.retried)
	}
}

func TestWorkerPausesOnRateLimit(t *testing.T) {
	q := &queue{retried: map[int64]bool{}, items: []app.OutboxItem{{ID: 1, Notification: app.Notification{UserID: 10}, Attempts: 1}}}
	c := &clock{now: time.Unix(0, 0)}
	w := jobs.NewOutboxWorker(q, func(context.Context, app.Notification) error { return limited{} },
		jobs.NewLimiter(0, 0, c.Now, c.Sleep), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := w.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(c.slept) == 0 {
		t.Fatal("worker must pause after 429")
	}
}
