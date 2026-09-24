package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
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

// cycleQueue — очередь для полного цикла Run: одна пачка, потом пусто; считает вызовы.
type cycleQueue struct {
	mu       sync.Mutex
	items    []app.OutboxItem
	released bool
	done     []int64
	claimErr error
}

func (q *cycleQueue) Claim(context.Context, int) ([]app.OutboxItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.claimErr != nil {
		err := q.claimErr
		q.claimErr = nil
		return nil, err
	}
	items := q.items
	q.items = nil
	return items, nil
}
func (q *cycleQueue) Done(_ context.Context, id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.done = append(q.done, id)
	return nil
}
func (q *cycleQueue) Retry(context.Context, int64, time.Time, string, bool) error { return nil }
func (q *cycleQueue) Release(context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.released = true
	return nil
}
func (q *cycleQueue) doneCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.done)
}

func TestWorkerRunReleasesProcessesIdlesAndStops(t *testing.T) {
	q := &cycleQueue{
		claimErr: errors.New("db hiccup"), // первая попытка падает: воркер не должен умереть
		items:    []app.OutboxItem{{ID: 7, Notification: app.Notification{UserID: 1}, Attempts: 1}},
	}
	w := jobs.NewOutboxWorker(q, func(context.Context, app.Notification) error { return nil },
		jobs.NewLimiter(0, 0, time.Now, jobs.SleepCtx), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	for q.doneCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if q.doneCount() != 1 || !q.released {
		t.Fatalf("done = %d, released = %v", q.doneCount(), q.released)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop while idle")
	}
}

func TestSleepCtx(t *testing.T) {
	if err := jobs.SleepCtx(t.Context(), time.Millisecond); err != nil {
		t.Fatalf("short sleep = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := jobs.SleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled sleep = %v", err)
	}
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
