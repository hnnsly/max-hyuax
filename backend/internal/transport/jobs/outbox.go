// Пакет jobs — фоновые задачи сервиса: отправка очереди сообщений бота (outbox).
package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"dommax/internal/app"
)

// Лимиты Bot API MAX: не больше 30 запросов в секунду всего и 2 в секунду на чат.
const (
	GlobalInterval  = time.Second / 30
	PerChatInterval = time.Second / 2
)

// Limiter выдерживает паузы между запросами: общую и отдельную для каждого чата.
type Limiter struct {
	mu      sync.Mutex
	global  time.Duration
	perKey  time.Duration
	last    time.Time
	lastKey map[int64]time.Time
	now     func() time.Time
	sleep   func(context.Context, time.Duration) error
}

func NewLimiter(global, perKey time.Duration, now func() time.Time, sleep func(context.Context, time.Duration) error) *Limiter {
	return &Limiter{global: global, perKey: perKey, lastKey: map[int64]time.Time{}, now: now, sleep: sleep}
}

// SleepCtx — пауза, прерываемая отменой контекста.
func SleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Wait ждёт, пока можно отправить следующий запрос в чат key.
func (l *Limiter) Wait(ctx context.Context, key int64) error {
	l.mu.Lock()
	now := l.now()
	at := latest(l.last.Add(l.global), l.lastKey[key].Add(l.perKey), now)
	l.last, l.lastKey[key] = at, at
	if len(l.lastKey) > 10_000 {
		// Старые чаты уже не ограничивают отправку: карта не растёт бесконечно.
		for k, t := range l.lastKey {
			if now.Sub(t) > l.perKey {
				delete(l.lastKey, k)
			}
		}
	}
	l.mu.Unlock()
	if d := at.Sub(now); d > 0 {
		return l.sleep(ctx, d)
	}
	return nil
}

func latest(ts ...time.Time) time.Time {
	out := ts[0]
	for _, t := range ts[1:] {
		if t.After(out) {
			out = t
		}
	}
	return out
}

// Pause — общая пауза после ответа «слишком много запросов».
func (l *Limiter) Pause(ctx context.Context, d time.Duration) error { return l.sleep(ctx, d) }

type Queue interface {
	Claim(ctx context.Context, n int) ([]app.OutboxItem, error)
	Done(ctx context.Context, id int64) error
	Retry(ctx context.Context, id int64, at time.Time, reason string, failed bool) error
	Release(ctx context.Context) error
}

const (
	batchSize   = 20
	maxAttempts = 8
	idlePause   = time.Second
)

// OutboxWorker отправляет уведомления из очереди с учётом лимитов и повторов.
type OutboxWorker struct {
	queue   Queue
	deliver func(context.Context, app.Notification) error
	limiter *Limiter
	log     *slog.Logger
}

func NewOutboxWorker(q Queue, deliver func(context.Context, app.Notification) error, l *Limiter, log *slog.Logger) *OutboxWorker {
	return &OutboxWorker{queue: q, deliver: deliver, limiter: l, log: log}
}

// Run обрабатывает очередь, пока ctx не отменён.
func (w *OutboxWorker) Run(ctx context.Context) error {
	if err := w.queue.Release(ctx); err != nil {
		w.log.WarnContext(ctx, "outbox release failed", "err", err)
	}
	for {
		n, err := w.RunOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			w.log.ErrorContext(ctx, "outbox batch failed", "err", err)
		}
		if n == 0 || err != nil {
			if err := SleepCtx(ctx, idlePause); err != nil {
				return err
			}
		}
	}
}

// RunOnce забирает и отправляет одну пачку; возвращает, сколько уведомлений обработано.
func (w *OutboxWorker) RunOnce(ctx context.Context) (int, error) {
	items, err := w.queue.Claim(ctx, batchSize)
	if err != nil {
		return 0, err
	}
	for _, it := range items {
		if err := w.limiter.Wait(ctx, it.UserID); err != nil {
			return 0, err
		}
		err := w.deliver(ctx, it.Notification)
		if err == nil {
			if err := w.queue.Done(ctx, it.ID); err != nil {
				return 0, err
			}
			continue
		}
		failed := it.Attempts >= maxAttempts
		// В лог — только вид уведомления и ошибка: без ПДн и текстов обращений.
		w.log.WarnContext(ctx, "outbox delivery failed", "id", it.ID, "kind", it.Kind, "attempt", it.Attempts, "failed", failed, "err", err)
		if err := w.queue.Retry(ctx, it.ID, time.Now().Add(backoff(it.Attempts)), err.Error(), failed); err != nil {
			return 0, err
		}
		if rl, ok := errors.AsType[interface {
			error
			RateLimited() bool
		}](err); ok && rl.RateLimited() {
			if err := w.limiter.Pause(ctx, time.Second); err != nil {
				return 0, err
			}
		}
	}
	return len(items), nil
}

// backoff — пауза перед повтором: 5 с, 10 с, 20 с и так далее, не больше 10 минут.
func backoff(attempt int) time.Duration {
	return min(5*time.Second<<max(attempt-1, 0), 10*time.Minute)
}
