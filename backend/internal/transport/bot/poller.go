package bot

import (
	"context"
	"log/slog"
	"time"

	"dommax/internal/storage/maxapi"
)

type Feed interface {
	Updates(ctx context.Context, marker int64, timeout time.Duration) (maxapi.UpdatesPage, error)
}

type UpdateHandler interface {
	Handle(ctx context.Context, u maxapi.Update) error
}

// Poller читает события через long polling. MAX рекомендует его только для разработки;
// в продакшене используется webhook (BOT_MODE=webhook).
type Poller struct {
	feed Feed
	h    UpdateHandler
	log  *slog.Logger
}

func NewPoller(feed Feed, h UpdateHandler, log *slog.Logger) *Poller {
	return &Poller{feed: feed, h: h, log: log}
}

const (
	pollTimeout  = 30 * time.Second
	retryBackoff = 3 * time.Second
)

// Run опрашивает MAX, пока ctx не отменён. Событие с ошибкой пишется в лог и пропускается,
// чтобы одно плохое событие не остановило бота.
func (p *Poller) Run(ctx context.Context) error {
	var marker int64
	for {
		page, err := p.feed.Updates(ctx, marker, pollTimeout)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			p.log.WarnContext(ctx, "poll updates failed", "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryBackoff):
			}
			continue
		}
		for _, u := range page.Updates {
			if err := p.h.Handle(ctx, u); err != nil {
				p.log.ErrorContext(ctx, "handle update failed", "type", u.Type, "err", err)
			}
		}
		if page.Marker != 0 {
			marker = page.Marker
		}
	}
}
