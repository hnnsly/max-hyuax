package bot_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"dommax/internal/storage/maxapi"
	"dommax/internal/transport/bot"
)

type memDedup struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (d *memDedup) MarkUpdateProcessed(_ context.Context, key string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen[key] {
		return false, nil
	}
	d.seen[key] = true
	return true, nil
}

type safeRecorder struct {
	mu  sync.Mutex
	got []maxapi.UpdateType
}

func (r *safeRecorder) Handle(_ context.Context, u maxapi.Update) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, u.Type)
	return nil
}

const startedJSON = `{"update_type":"bot_started","timestamp":1790029657169,"chat_id":7,"user":{"user_id":1001}}`

func newWebhook() (*bot.Webhook, *safeRecorder) {
	rec := &safeRecorder{}
	w := bot.NewWebhook("s3cret", rec, &memDedup{seen: map[string]bool{}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return w, rec
}

func TestWebhookHandlesUpdateOnceDespiteRedelivery(t *testing.T) {
	w, rec := newWebhook()
	for range 2 {
		if err := w.Accept(t.Context(), "s3cret", []byte(startedJSON)); err != nil {
			t.Fatalf("Accept: %v", err)
		}
	}
	w.Wait()
	if len(rec.got) != 1 || rec.got[0] != maxapi.UpdateBotStarted {
		t.Fatalf("handled = %v, want one bot_started", rec.got)
	}
}

func TestWebhookRejectsWrongSecret(t *testing.T) {
	w, rec := newWebhook()
	if err := w.Accept(t.Context(), "guess", []byte(startedJSON)); !errors.Is(err, bot.ErrBadSecret) {
		t.Fatalf("err = %v, want ErrBadSecret", err)
	}
	w.Wait()
	if len(rec.got) != 0 {
		t.Fatalf("handled = %v", rec.got)
	}
}

func TestWebhookRejectsMalformedBody(t *testing.T) {
	w, _ := newWebhook()
	if err := w.Accept(t.Context(), "s3cret", []byte(`{`)); !errors.Is(err, bot.ErrBadUpdate) {
		t.Fatalf("err = %v, want ErrBadUpdate", err)
	}
}

func TestUpdateKeyDistinguishesEvents(t *testing.T) {
	a := bot.UpdateKey(maxapi.Update{Type: maxapi.UpdateMessageCallback, Timestamp: 1, Callback: &maxapi.Callback{ID: "cb1"}})
	b := bot.UpdateKey(maxapi.Update{Type: maxapi.UpdateMessageCallback, Timestamp: 1, Callback: &maxapi.Callback{ID: "cb2"}})
	c := bot.UpdateKey(maxapi.Update{Type: maxapi.UpdateMessageCreated, Timestamp: 1, Message: &maxapi.Message{Body: maxapi.MessageBody{MID: "m1"}}})
	if a == b || a == c || b == c {
		t.Fatalf("keys collide: %q %q %q", a, b, c)
	}
}
