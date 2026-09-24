package jobs_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"dommax/internal/transport/jobs"
)

// Задача проверяет просрочку сразу при старте, затем по интервалу и выходит по отмене.
func TestOverdueJobRunsAtStartAndOnTick(t *testing.T) {
	var calls atomic.Int32
	mark := func(context.Context) (int, error) {
		if calls.Add(1) == 2 {
			return 0, errors.New("db down") // ошибка прохода не останавливает задачу
		}
		return 1, nil
	}
	job := jobs.NewOverdueJob(mark, 5*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- job.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if calls.Load() < 3 {
		t.Fatalf("calls = %d, want at least 3", calls.Load())
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job did not stop")
	}
}

func TestOverdueJobChecksBeforeFirstTick(t *testing.T) {
	var calls atomic.Int32
	job := jobs.NewOverdueJob(func(context.Context) (int, error) { calls.Add(1); return 0, nil }, time.Hour,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- job.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (only the start check)", calls.Load())
	}
}
