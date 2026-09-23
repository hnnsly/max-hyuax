package bot_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"dommax/internal/storage/maxapi"
	"dommax/internal/transport/bot"
)

type fakeFeed struct {
	pages   []maxapi.UpdatesPage
	markers []int64
	cancel  context.CancelFunc
}

func (f *fakeFeed) Updates(_ context.Context, marker int64, _ time.Duration) (maxapi.UpdatesPage, error) {
	f.markers = append(f.markers, marker)
	if len(f.pages) == 0 {
		f.cancel()
		return maxapi.UpdatesPage{}, context.Canceled
	}
	p := f.pages[0]
	f.pages = f.pages[1:]
	return p, nil
}

type recorder struct{ got []maxapi.UpdateType }

func (r *recorder) Handle(_ context.Context, u maxapi.Update) error {
	r.got = append(r.got, u.Type)
	if u.Type == "boom" {
		return errors.New("handler failed")
	}
	return nil
}

func TestPollerAdvancesMarkerAndSurvivesHandlerErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	feed := &fakeFeed{cancel: cancel, pages: []maxapi.UpdatesPage{
		{Marker: 10, Updates: []maxapi.Update{{Type: "boom"}, {Type: maxapi.UpdateBotStarted}}},
		{Marker: 11, Updates: []maxapi.Update{{Type: maxapi.UpdateMessageCreated}}},
	}}
	rec := &recorder{}
	p := bot.NewPoller(feed, rec, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := p.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v, want context.Canceled", err)
	}
	if want := []int64{0, 10, 11}; !slices.Equal(feed.markers, want) {
		t.Errorf("markers = %v, want %v", feed.markers, want)
	}
	if len(rec.got) != 3 {
		t.Errorf("handled = %v, want all 3 updates despite the error", rec.got)
	}
}
