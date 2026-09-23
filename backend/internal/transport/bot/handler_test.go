package bot_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"dommax/internal/app/apptest"
	"dommax/internal/app/auth"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
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

// last — текст и кнопки последнего ответа бота: отправленного сообщения или ответа на нажатие.
func (f *fakeMax) last(cbID string) (string, []maxapi.Button) {
	if a, ok := f.answers[cbID]; ok && cbID != "" {
		if a.Message == nil {
			return a.Notification, nil
		}
		return a.Message.Text, buttons(*a.Message)
	}
	if len(f.sent) == 0 {
		return "", nil
	}
	m := f.sent[len(f.sent)-1].msg
	return m.Text, buttons(m)
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

func findButton(t *testing.T, bs []maxapi.Button, text string) maxapi.Button {
	t.Helper()
	for _, b := range bs {
		if b.Text == text {
			return b
		}
	}
	t.Fatalf("no button %q in %+v", text, bs)
	return maxapi.Button{}
}

const botName = "t105_hakaton_max_bot"

type env struct {
	store *apptest.MemStore
	max   *fakeMax
	h     *bot.Handler
}

func newEnv(t *testing.T) env {
	t.Helper()
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК «Ореховый квартал»", PhoneDispatcher: "+7 495 000-17-02"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	now := func() time.Time { return time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC) }
	n := 0
	svc := bot.Services{
		Auth:           auth.NewService(s, auth.Config{Now: now}),
		Issues:         issues.NewService(s, issues.Config{Now: now, NewID: func() string { n++; return fmt.Sprintf("i-%d", n) }, ConsentVersion: "v1"}),
		Houses:         houses.NewService(s),
		ConsentVersion: "v1",
		Now:            now,
	}
	f := &fakeMax{}
	return env{store: s, max: f, h: bot.NewHandler(f, botName, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))}
}

func text(from int64, body string) maxapi.Update {
	return maxapi.Update{Type: maxapi.UpdateMessageCreated, Message: &maxapi.Message{
		Sender:    maxapi.User{UserID: from, FirstName: "Анна"},
		Recipient: maxapi.Recipient{ChatID: 7, ChatType: "dialog"},
		Body:      maxapi.MessageBody{Text: body},
	}}
}

func press(from int64, id, payload string) maxapi.Update {
	return maxapi.Update{Type: maxapi.UpdateMessageCallback,
		Callback: &maxapi.Callback{ID: id, Payload: payload, User: maxapi.User{UserID: from, FirstName: "Анна"}},
		Message:  &maxapi.Message{Recipient: maxapi.Recipient{ChatID: 7, ChatType: "dialog"}},
	}
}

func (e env) handle(t *testing.T, u maxapi.Update) {
	t.Helper()
	if err := e.h.Handle(t.Context(), u); err != nil {
		t.Fatalf("Handle: %v", err)
	}
}

// resident — житель MAX с домом и согласием.
func (e env) resident(maxID int64, consent bool) user.User {
	u := user.User{ID: maxID, MaxUserID: maxID, Role: user.RoleResident, HouseID: "h-1"}
	if consent {
		u.ConsentVersion = "v1"
	}
	e.store.AddUser(u)
	return u
}

func TestStartSendsGreetingWithEntryButtons(t *testing.T) {
	e := newEnv(t)
	e.handle(t, maxapi.Update{Type: maxapi.UpdateBotStarted, ChatID: 7, User: maxapi.User{UserID: 1001}})
	txt, bs := e.max.last("")
	if !strings.Contains(txt, "заявк") || e.max.sent[0].to != maxapi.ToChat(7) {
		t.Fatalf("greeting = %q to %+v", txt, e.max.sent[0].to)
	}
	findButton(t, bs, "Сообщить о проблеме")
	if b := findButton(t, bs, "Открыть приложение"); b.WebApp != botName {
		t.Fatalf("open_app = %+v", b)
	}
}

func TestGroupChatsAndBotsAreIgnored(t *testing.T) {
	e := newEnv(t)
	u := text(1001, "/start")
	u.Message.Recipient.ChatType = "chat"
	e.handle(t, u)
	u = text(1001, "/start")
	u.Message.Sender.IsBot = true
	e.handle(t, u)
	e.handle(t, maxapi.Update{Type: "dialog_muted"})
	if len(e.max.sent) != 0 || len(e.max.answers) != 0 {
		t.Fatalf("bot must stay silent: %+v %+v", e.max.sent, e.max.answers)
	}
}

func TestWithoutHouseBotAsksLocationThenBindsHouse(t *testing.T) {
	e := newEnv(t)
	e.handle(t, text(2001, "не горит свет на 5 этаже"))
	_, bs := e.max.last("")
	if findButton(t, bs, "Отправить геопозицию").Type != "request_geo_location" {
		t.Fatalf("buttons = %+v", bs)
	}

	loc := text(2001, "")
	loc.Message.Body.Attachments = []maxapi.IncomingAttachment{{Type: "location", Latitude: 55.61, Longitude: 37.74}}
	e.handle(t, loc)
	_, bs = e.max.last("")
	pick := findButton(t, bs, "Ореховый бульвар, 17к2")

	e.handle(t, press(2001, "cb1", pick.Payload))
	u, err := e.store.Users().ByMaxID(t.Context(), 2001)
	if err != nil || u.HouseID != "h-1" {
		t.Fatalf("user = %+v, err = %v", u, err)
	}
	if txt, _ := e.max.last("cb1"); !strings.Contains(txt, "опишите проблему") {
		t.Fatalf("after house = %q", txt)
	}
}

func TestProblemIsConfirmedThenReported(t *testing.T) {
	e := newEnv(t)
	e.resident(3001, true)
	e.handle(t, text(3001, "Не горит свет на лестнице"))
	txt, bs := e.max.last("")
	if !strings.Contains(txt, "Свет в подъезде") || !strings.Contains(txt, "УК «Ореховый квартал»") {
		t.Fatalf("confirm = %q", txt)
	}
	e.handle(t, press(3001, "cb2", findButton(t, bs, "Отправить").Payload))
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)
	if len(list) != 1 || list[0].Category() != "lighting" || list[0].Description() != "Не горит свет на лестнице" {
		t.Fatalf("issues = %+v", list)
	}
	if txt, _ := e.max.last("cb2"); !strings.Contains(txt, fmt.Sprintf("Заявка № %d", list[0].Number())) {
		t.Fatalf("answer = %q", txt)
	}
}

func TestCategoryCanBeChanged(t *testing.T) {
	e := newEnv(t)
	e.resident(3002, true)
	e.handle(t, text(3002, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(3002, "cb1", findButton(t, bs, "Изменить").Payload))
	_, bs = e.max.last("cb1")
	e.handle(t, press(3002, "cb2", findButton(t, bs, "Дверь подъезда и домофон").Payload))
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)
	if len(list) != 1 || list[0].Category() != "door" {
		t.Fatalf("issues = %+v", list)
	}
}

func TestSimilarIssueOffersJoin(t *testing.T) {
	e := newEnv(t)
	e.resident(4001, true)
	e.resident(4002, true)
	e.handle(t, text(4001, "Лифт не работает"))
	_, bs := e.max.last("")
	e.handle(t, press(4001, "cb1", findButton(t, bs, "Отправить").Payload))

	e.handle(t, text(4002, "лифт опять стоит"))
	txt, bs := e.max.last("")
	if !strings.Contains(txt, "уже сообщили") {
		t.Fatalf("offer = %q", txt)
	}
	e.handle(t, press(4002, "cb2", findButton(t, bs, "Это и у меня").Payload))
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)
	if len(list) != 1 || list[0].ParticipantCount() != 2 {
		t.Fatalf("issues = %+v", list)
	}
}

func TestUnclearTextOpensForm(t *testing.T) {
	e := newEnv(t)
	e.resident(5001, true)
	e.handle(t, text(5001, "Что-то непонятное происходит во дворе"))
	_, bs := e.max.last("")
	if b := findButton(t, bs, "Открыть форму"); b.Type != "open_app" {
		t.Fatalf("button = %+v", b)
	}
}

func TestConsentIsAskedBeforeReport(t *testing.T) {
	e := newEnv(t)
	e.resident(6001, false)
	e.handle(t, text(6001, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(6001, "cb1", findButton(t, bs, "Отправить").Payload))
	txt, bs := e.max.last("cb1")
	if !strings.Contains(txt, "согласие") {
		t.Fatalf("consent prompt = %q", txt)
	}
	e.handle(t, press(6001, "cb2", findButton(t, bs, "Согласен").Payload))
	u, _ := e.store.Users().ByMaxID(t.Context(), 6001)
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)
	if !u.HasConsent("v1") || len(list) != 1 {
		t.Fatalf("consent = %v, issues = %d", u.HasConsent("v1"), len(list))
	}
}

func TestMyAndHouseCommands(t *testing.T) {
	e := newEnv(t)
	e.resident(7001, true)
	e.handle(t, text(7001, "/my"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Заявок пока нет") {
		t.Fatalf("/my empty = %q", txt)
	}
	e.handle(t, text(7001, "/house"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "+7 495 000-17-02") || !strings.Contains(txt, "Ореховый бульвар, 17к2") {
		t.Fatalf("/house = %q", txt)
	}
}
