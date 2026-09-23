package bot_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"dommax/internal/storage/maxapi"
	"dommax/internal/transport/bot"
)

type sent struct {
	to  maxapi.Target
	msg maxapi.NewMessage
}

type fakeMax struct {
	sent    []sent
	answers map[string]maxapi.CallbackAnswer
}

func (f *fakeMax) Send(_ context.Context, to maxapi.Target, m maxapi.NewMessage) (maxapi.Message, error) {
	f.sent = append(f.sent, sent{to, m})
	return maxapi.Message{}, nil
}

func (f *fakeMax) Answer(_ context.Context, id string, a maxapi.CallbackAnswer) error {
	if f.answers == nil {
		f.answers = map[string]maxapi.CallbackAnswer{}
	}
	f.answers[id] = a
	return nil
}

func newHandler() (*bot.Handler, *fakeMax) {
	f := &fakeMax{}
	return bot.NewHandler(f, "t105_hakaton_max_bot", slog.New(slog.NewTextHandler(io.Discard, nil))), f
}

func buttons(m maxapi.NewMessage) []maxapi.Button {
	var out []maxapi.Button
	for _, a := range m.Attachments {
		for _, row := range a.Payload.(maxapi.KeyboardPayload).Buttons {
			out = append(out, row...)
		}
	}
	return out
}

func assertGreeting(t *testing.T, f *fakeMax, want maxapi.Target) {
	t.Helper()
	if len(f.sent) != 1 {
		t.Fatalf("sent = %d messages, want 1", len(f.sent))
	}
	s := f.sent[0]
	if s.to != want {
		t.Errorf("target = %+v, want %+v", s.to, want)
	}
	if !strings.Contains(s.msg.Text, "заявк") {
		t.Errorf("greeting text = %q", s.msg.Text)
	}
	var report, app bool
	for _, b := range buttons(s.msg) {
		report = report || (b.Type == "callback" && b.Payload == bot.PayloadReport)
		app = app || (b.Type == "open_app" && b.WebApp == "t105_hakaton_max_bot")
	}
	if !report || !app {
		t.Errorf("greeting buttons = %+v, want report callback and open_app", buttons(s.msg))
	}
}

func TestBotStartedSendsGreeting(t *testing.T) {
	h, f := newHandler()
	err := h.Handle(t.Context(), maxapi.Update{Type: maxapi.UpdateBotStarted, ChatID: 7, User: maxapi.User{UserID: 1001}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	assertGreeting(t, f, maxapi.ToChat(7))
}

func TestStartCommandSendsGreeting(t *testing.T) {
	h, f := newHandler()
	err := h.Handle(t.Context(), maxapi.Update{Type: maxapi.UpdateMessageCreated, Message: &maxapi.Message{
		Sender:    maxapi.User{UserID: 1001},
		Recipient: maxapi.Recipient{ChatID: 7, ChatType: "dialog"},
		Body:      maxapi.MessageBody{Text: " /start "},
	}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	assertGreeting(t, f, maxapi.ToChat(7))
}

func TestGroupChatMessagesAreIgnored(t *testing.T) {
	h, f := newHandler()
	err := h.Handle(t.Context(), maxapi.Update{Type: maxapi.UpdateMessageCreated, Message: &maxapi.Message{
		Sender:    maxapi.User{UserID: 1001},
		Recipient: maxapi.Recipient{ChatID: 9, ChatType: "chat"},
		Body:      maxapi.MessageBody{Text: "/start"},
	}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(f.sent) != 0 {
		t.Fatalf("bot must stay silent in group chats, sent %+v", f.sent)
	}
}

func TestMessagesFromBotsAreIgnored(t *testing.T) {
	h, f := newHandler()
	_ = h.Handle(t.Context(), maxapi.Update{Type: maxapi.UpdateMessageCreated, Message: &maxapi.Message{
		Sender:    maxapi.User{UserID: 5, IsBot: true},
		Recipient: maxapi.Recipient{ChatID: 7, ChatType: "dialog"},
		Body:      maxapi.MessageBody{Text: "/start"},
	}})
	if len(f.sent) != 0 {
		t.Fatalf("sent = %+v", f.sent)
	}
}

func TestReportCallbackIsAnswered(t *testing.T) {
	h, f := newHandler()
	err := h.Handle(t.Context(), maxapi.Update{Type: maxapi.UpdateMessageCallback,
		Callback: &maxapi.Callback{ID: "cb1", Payload: bot.PayloadReport, User: maxapi.User{UserID: 1001}},
		Message:  &maxapi.Message{Recipient: maxapi.Recipient{ChatID: 7, ChatType: "dialog"}},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if a, ok := f.answers["cb1"]; !ok || a.Notification == "" {
		t.Fatalf("answers = %+v, want a notification for cb1", f.answers)
	}
}

func TestUnknownUpdateIsIgnored(t *testing.T) {
	h, f := newHandler()
	if err := h.Handle(t.Context(), maxapi.Update{Type: "dialog_muted"}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(f.sent) != 0 || len(f.answers) != 0 {
		t.Fatal("unknown update must not produce output")
	}
}
