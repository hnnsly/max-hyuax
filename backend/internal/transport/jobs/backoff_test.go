package jobs

import (
	"context"
	"testing"
	"time"
)

func TestBackoffGrowsAndCaps(t *testing.T) {
	cases := map[int]time.Duration{0: 5 * time.Second, 1: 5 * time.Second, 2: 10 * time.Second, 3: 20 * time.Second, 20: 10 * time.Minute}
	for attempt, want := range cases {
		if got := backoff(attempt); got != want {
			t.Errorf("backoff(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestLimiterForgetsOldChats(t *testing.T) {
	now := time.Unix(0, 0)
	sleep := func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil }
	l := NewLimiter(0, time.Second, func() time.Time { return now }, sleep)
	for i := range int64(10_002) {
		_ = l.Wait(t.Context(), i)
		now = now.Add(2 * time.Second)
	}
	// Чаты, из которых давно ничего не отправляли, не должны копиться в памяти.
	if len(l.lastKey) > 10 {
		t.Fatalf("lastKey keeps %d stale chats", len(l.lastKey))
	}
}
