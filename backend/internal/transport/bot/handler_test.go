package bot_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/auth"
	"dommax/internal/app/cards"
	appcouncil "dommax/internal/app/council"
	"dommax/internal/app/hints"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/app/photos"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
	"dommax/internal/storage/maxapi"
	"dommax/internal/transport/bot"
)

type sent struct {
	to  maxapi.Target
	msg maxapi.NewMessage
}

type fakeMax struct {
	sent       []sent
	answers    map[string]maxapi.CallbackAnswer
	downloaded []string
}

// Download отдаёт маленький JPEG по любой ссылке и запоминает её.
func (f *fakeMax) Download(_ context.Context, url string, _ int64) ([]byte, error) {
	f.downloaded = append(f.downloaded, url)
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 32, 24)), nil)
	return buf.Bytes(), nil
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

const (
	botName      = "t105_hakaton_max_bot"
	testBotToken = "test-bot-token"
)

type env struct {
	store   *apptest.MemStore
	max     *fakeMax
	h       *bot.Handler
	issues  *issues.Service
	council *appcouncil.Service
}

func newEnv(t *testing.T) env { return newEnvWithLLM(t, nil) }

// newEnvWithLLM — окружение бота с моделью для подсказки категории (nil — только правила).
func newEnvWithLLM(t *testing.T, llm hints.LLM) env {
	t.Helper()
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК «Ореховый квартал»", PhoneDispatcher: "+7 495 000-17-02"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	now := func() time.Time { return time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC) }
	n := 0
	svc := bot.Services{
		Auth:           auth.NewService(s, auth.Config{Now: now, BotToken: testBotToken}),
		Issues:         issues.NewService(s, issues.Config{Now: now, NewID: func() string { n++; return fmt.Sprintf("i-%d", n) }, ConsentVersion: "v1"}),
		Houses:         houses.NewService(s, nil),
		Hints:          hints.NewService(llm, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))),
		Photos:         photos.NewService(s, &apptest.MemFiles{}, photos.Config{Now: now, NewID: func() string { n++; return fmt.Sprintf("p-%d", n) }, ConsentVersion: "v1"}),
		Cards:          cards.NewService(s, nil, now),
		Council:        appcouncil.NewService(s, appcouncil.Config{Now: now, NewID: func() string { n++; return fmt.Sprintf("c-%d", n) }, ConsentVersion: "v1"}),
		Pending:        s.Pending(),
		ConsentVersion: "v1",
		Now:            now,
	}
	f := &fakeMax{}
	return env{store: s, max: f, issues: svc.Issues, council: svc.Council, h: bot.NewHandler(f, botName, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))}
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
	if findButton(t, bs, "Показать дома рядом").Type != "request_geo_location" {
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

func photoMessage(from int64, url string) maxapi.Update {
	u := text(from, "")
	payload := `{"photo_id":7,"token":"t"}`
	if url != "" {
		payload = `{"photo_id":7,"token":"t","url":"` + url + `"}`
	}
	u.Message.Body.Attachments = []maxapi.IncomingAttachment{{Type: "image", Payload: jsontext.Value(payload)}}
	return u
}

// Фото после заявки прикрепляется к последней открытой заявке жителя (FR-BOT-04).
func TestPhotoIsAttachedToLatestIssue(t *testing.T) {
	e := newEnv(t)
	e.resident(3020, true)
	e.handle(t, text(3020, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(3020, "cb1", findButton(t, bs, "Отправить").Payload))
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)

	e.handle(t, photoMessage(3020, "https://i.oneme.ru/photo.jpg"))
	photos, _ := e.store.Photos().ListByIssue(t.Context(), list[0].ID())
	if len(photos) != 1 || len(e.max.downloaded) != 1 {
		t.Fatalf("photos = %+v, downloaded = %v", photos, e.max.downloaded)
	}
	txt, bs := e.max.last("")
	if !strings.Contains(txt, fmt.Sprintf("Фото добавлено к заявке № %d", list[0].Number())) {
		t.Fatalf("reply = %q", txt)
	}
	findButton(t, bs, "Открыть заявку")
}

// «Починили» в сообщении о выполнении подтверждает ремонт; повторное нажатие отвечает, что ответ уже есть.
func TestConfirmRepairFromBot(t *testing.T) {
	e := newEnv(t)
	e.resident(3030, true)
	e.handle(t, text(3030, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(3030, "cb1", findButton(t, bs, "Отправить").Payload))
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)
	is := list[0]
	oper := user.User{ID: 9001, Role: user.RoleOperator, OrganizationID: "org-1"}
	e.store.AddUser(oper)
	svc := issues.NewService(e.store, issues.Config{Now: time.Now, NewID: func() string { return "unused" }, ConsentVersion: "v1"})
	for _, st := range []issue.Status{issue.StatusInProgress, issue.StatusDone} {
		if _, err := svc.ChangeStatus(t.Context(), oper, is.ID(), st, "Заменили лампы"); err != nil {
			t.Fatal(err)
		}
	}
	payload := bot.ConfirmPayload(is.ID())
	e.handle(t, press(3030, "cb2", payload))
	if got, _ := e.store.Issues().Get(t.Context(), is.ID()); got.ConfirmedCount() != 1 {
		t.Fatalf("confirmed = %d, want 1", got.ConfirmedCount())
	}
	if txt, _ := e.max.last("cb2"); !strings.Contains(txt, "Спасибо") {
		t.Fatalf("answer = %q", txt)
	}
	e.handle(t, press(3030, "cb3", payload))
	if txt, _ := e.max.last("cb3"); !strings.Contains(txt, "уже ответили") {
		t.Fatalf("second answer = %q", txt)
	}
}

// Альбом из нескольких снимков прикладывается целиком, а не только первое фото.
func TestAlbumPhotosAreAttachedTogether(t *testing.T) {
	e := newEnv(t)
	e.resident(3022, true)
	e.handle(t, text(3022, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(3022, "cb1", findButton(t, bs, "Отправить").Payload))
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)

	album := photoMessage(3022, "https://i.oneme.ru/1.jpg")
	album.Message.Body.Attachments = append(album.Message.Body.Attachments, maxapi.IncomingAttachment{
		Type: "image", Payload: jsontext.Value(`{"photo_id":8,"token":"t","url":"https://i.oneme.ru/2.jpg"}`),
	})
	e.handle(t, album)
	photos, _ := e.store.Photos().ListByIssue(t.Context(), list[0].ID())
	if len(photos) != 2 || len(e.max.downloaded) != 2 {
		t.Fatalf("photos = %d, downloaded = %v", len(photos), e.max.downloaded)
	}
	if txt, _ := e.max.last(""); !strings.Contains(txt, fmt.Sprintf("2 фото добавлены к заявке № %d", list[0].Number())) {
		t.Fatalf("reply = %q", txt)
	}
}

// Согласие устарело (новая версия документа): бот просит его, а не прикладывает фото молча.
func TestPhotoAsksForConsent(t *testing.T) {
	e := newEnv(t)
	u := e.resident(3023, true)
	e.handle(t, text(3023, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(3023, "cb1", findButton(t, bs, "Отправить").Payload))
	u.ConsentVersion = ""
	e.store.AddUser(u)

	e.handle(t, photoMessage(3023, "https://i.oneme.ru/photo.jpg"))
	txt, bs := e.max.last("")
	if !strings.Contains(txt, "согласие на обработку персональных данных") {
		t.Fatalf("reply = %q", txt)
	}
	e.handle(t, press(3023, "cb2", findButton(t, bs, "Согласен").Payload))
	if got, _ := e.store.Users().Get(t.Context(), u.ID); !got.HasConsent("v1") {
		t.Fatalf("consent not saved: %+v", got)
	}
	if txt, _ := e.max.last("cb2"); !strings.Contains(txt, "Пришлите фото ещё раз") {
		t.Fatalf("answer = %q", txt)
	}
}

func TestPhotoWithoutIssueOrLink(t *testing.T) {
	e := newEnv(t)
	e.resident(3021, true)
	e.handle(t, photoMessage(3021, "https://i.oneme.ru/photo.jpg"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Сначала опишите проблему") || len(e.max.downloaded) != 0 {
		t.Fatalf("no issue: reply = %q, downloaded = %v", txt, e.max.downloaded)
	}

	e.handle(t, text(3021, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(3021, "cb1", findButton(t, bs, "Отправить").Payload))
	for _, url := range []string{"", "http://insecure.example/p.jpg"} {
		e.handle(t, photoMessage(3021, url))
		if txt, _ := e.max.last(""); !strings.Contains(txt, "Не получилось получить фото") {
			t.Fatalf("url %q: reply = %q", url, txt)
		}
	}
	if len(e.max.downloaded) != 0 {
		t.Fatalf("downloaded = %v, want nothing", e.max.downloaded)
	}
}

type llmStub string

func (s llmStub) Category(context.Context, string, []rules.Rule) (string, error) {
	return string(s), nil
}

// Ключевых слов в тексте нет, но модель узнала протечку: бот предлагает её подтвердить.
func TestModelHintIsUsedWhenKeywordsMiss(t *testing.T) {
	e := newEnvWithLLM(t, llmStub("leak"))
	e.resident(3010, true)
	e.handle(t, text(3010, "Вода льётся по стене в подъезде"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Поняли так: Протечка") {
		t.Fatalf("confirm = %q", txt)
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

func TestStaleAndBrokenButtonsAreExplained(t *testing.T) {
	e := newEnv(t)
	e.resident(8001, true)
	cases := map[string]string{
		"x:whatever": "устарела",
		"h:nope":     "Не получилось",
		"j:nope":     "не найдена",
	}
	i := 0
	for payload, want := range cases {
		i++
		id := fmt.Sprintf("cb%d", i)
		e.handle(t, press(8001, id, payload))
		if txt, _ := e.max.last(id); !strings.Contains(txt, want) {
			t.Errorf("payload %q: answer %q, want %q", payload, txt, want)
		}
	}
}

func TestJoinClosedOrAlreadyJoinedIssue(t *testing.T) {
	e := newEnv(t)
	e.resident(8101, true)
	e.resident(8102, true)
	e.handle(t, text(8101, "Лифт не работает"))
	_, bs := e.max.last("")
	e.handle(t, press(8101, "cb1", findButton(t, bs, "Отправить").Payload))
	list, _ := e.store.Issues().ListByHouse(t.Context(), "h-1", 10)
	id := list[0].ID()

	e.handle(t, press(8101, "cb2", "j:"+id))
	if txt, _ := e.max.last("cb2"); !strings.Contains(txt, "уже среди сообщивших") {
		t.Fatalf("second join by reporter = %q", txt)
	}

	oper := user.User{ID: 8199, Role: user.RoleOperator, OrganizationID: "org-1"}
	e.store.AddUser(oper)
	svc := issues.NewService(e.store, issues.Config{Now: time.Now, NewID: func() string { return "unused" }, ConsentVersion: "v1"})
	for _, st := range []issue.Status{issue.StatusInProgress, issue.StatusDone} {
		if _, err := svc.ChangeStatus(t.Context(), oper, id, st, "Починили"); err != nil {
			t.Fatal(err)
		}
	}
	e.handle(t, press(8102, "cb3", "j:"+id))
	if txt, _ := e.max.last("cb3"); !strings.Contains(txt, "уже закрыта") {
		t.Fatalf("join closed = %q", txt)
	}
}

func TestCommandsAndReportButton(t *testing.T) {
	e := newEnv(t)
	e.handle(t, text(8201, "/help"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "/new") {
		t.Fatalf("/help = %q", txt)
	}
	e.handle(t, text(8201, "/new"))
	if _, bs := e.max.last(""); findButton(t, bs, "Показать дома рядом").Type != "request_geo_location" {
		t.Fatal("/new without house must ask for location")
	}
	e.handle(t, text(8201, "/unknown"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Здравствуйте") {
		t.Fatalf("unknown command = %q", txt)
	}
	sentBefore := len(e.max.sent)
	e.handle(t, press(8201, "cb1", bot.PayloadReport))
	if txt, _ := e.max.last("cb1"); !strings.Contains(txt, "Сначала укажите дом") || len(e.max.sent) != sentBefore+1 {
		t.Fatalf("report without house: answer %q, sent %d", txt, len(e.max.sent)-sentBefore)
	}

	e.resident(8202, true)
	e.handle(t, text(8202, "/new"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Опишите проблему") {
		t.Fatalf("/new with house = %q", txt)
	}
	e.handle(t, press(8202, "cb2", bot.PayloadReport))
	if txt, _ := e.max.last("cb2"); !strings.Contains(txt, "Опишите проблему") {
		t.Fatalf("report with house = %q", txt)
	}
}

func TestEmptyMessageAndLocationWithoutHouses(t *testing.T) {
	e := newEnv(t)
	e.handle(t, text(8301, "   "))
	if len(e.max.sent) != 0 {
		t.Fatalf("empty text must be ignored, sent %+v", e.max.sent)
	}
	delete(e.store.HouseMap, "h-1")
	loc := text(8301, "")
	loc.Message.Body.Attachments = []maxapi.IncomingAttachment{{Type: "location", Latitude: 10, Longitude: 10}}
	e.handle(t, loc)
	if txt, _ := e.max.last(""); !strings.Contains(txt, "за пределами Москвы") && !strings.Contains(txt, "не нашёлся") {
		t.Fatalf("no houses nearby = %q", txt)
	}
}

// «Мои заявки» открывают карточку прямо в чате: статус, срок, хронология и кнопки.
func TestMyCommandOpensIssueCardInChat(t *testing.T) {
	e := newEnv(t)
	e.resident(8401, true)
	e.handle(t, text(8401, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(8401, "cb1", findButton(t, bs, "Отправить").Payload))
	e.handle(t, text(8401, "/my"))
	txt, bs := e.max.last("")
	if !strings.Contains(txt, "Ваши заявки") || len(bs) != 1 || bs[0].Type != "callback" || !strings.HasPrefix(bs[0].Payload, "i:") {
		t.Fatalf("/my = %q %+v", txt, bs)
	}
	e.handle(t, press(8401, "cb2", bs[0].Payload))
	card, cbs := e.max.last("")
	if !strings.Contains(card, "Заявка № 101") || !strings.Contains(card, "Хронология") || !strings.Contains(card, "заявка подана") {
		t.Fatalf("card = %q", card)
	}
	findButton(t, cbs, "Открыть в приложении")
	// Автор уже участник: кнопки «Это и у меня» у него нет.
	for _, b := range cbs {
		if b.Text == "Это и у меня" {
			t.Fatal("author sees join button")
		}
	}
}

// Меню: /menu и пункты «Сейчас в доме» и «Мой дом».
func TestMenuShowsHouseIssues(t *testing.T) {
	e := newEnv(t)
	e.resident(8601, true)
	e.handle(t, text(8601, "/menu"))
	_, bs := e.max.last("")
	e.handle(t, press(8601, "cb1", findButton(t, bs, "Сейчас в доме").Payload))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "нет открытых заявок") {
		t.Fatalf("empty house = %q", txt)
	}
	e.handle(t, press(8601, "cb2", findButton(t, bs, "Мой дом").Payload))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Ореховый бульвар, 17к2") {
		t.Fatalf("my house = %q", txt)
	}
}

// «Не починили» в боте: бот спрашивает комментарий, следующее сообщение возвращает заявку в работу,
// а не создаёт новую проблему.
func TestReopenAsksCommentInChat(t *testing.T) {
	e := newEnv(t)
	anna := e.resident(8701, true)
	e.handle(t, text(8701, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(8701, "cb1", findButton(t, bs, "Отправить").Payload))
	oper := user.User{ID: 900, Role: user.RoleOperator, OrganizationID: "org-1"}
	e.store.AddUser(oper)
	for _, st := range []issue.Status{issue.StatusInProgress, issue.StatusDone} {
		if _, err := e.issues.ChangeStatus(t.Context(), oper, "i-1", st, ""); err != nil {
			t.Fatal(err)
		}
	}
	e.handle(t, press(8701, "cb2", "i:i-1"))
	_, cbs := e.max.last("")
	e.handle(t, press(8701, "cb3", findButton(t, cbs, "Не починили").Payload))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "что осталось не так") {
		t.Fatalf("reopen prompt = %q", txt)
	}
	e.handle(t, text(8701, "На третьем этаже всё ещё темно"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "снова в работе") {
		t.Fatalf("after comment = %q", txt)
	}
	is, _ := e.issues.Get(t.Context(), "i-1")
	if is.Status() != issue.StatusInProgress || !is.HasParticipant(anna.ID) {
		t.Fatalf("issue after reopen = %v", is.Status())
	}
	// Следующий текст снова считается новой проблемой.
	e.handle(t, text(8701, "не горит свет в подъезде"))
	if txt, _ := e.max.last(""); strings.Contains(txt, "снова в работе") {
		t.Fatalf("pending was not cleared: %q", txt)
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

// Свет бывает в каждом подъезде: бот спрашивает, в каком, и заявка получает место.
func TestReportAsksEntrance(t *testing.T) {
	e := newEnv(t)
	e.resident(8801, true)
	e.store.Objects = []house.AssetObject{
		{ID: "h-1-e1-light", HouseID: "h-1", EntranceID: "h-1-e1", Category: "lighting", Label: "подъезд 1, лестничная клетка"},
		{ID: "h-1-e2-light", HouseID: "h-1", EntranceID: "h-1-e2", Category: "lighting", Label: "подъезд 2, лестничная клетка"},
	}
	e.handle(t, text(8801, "Не горит свет на лестнице"))
	_, bs := e.max.last("")
	e.handle(t, press(8801, "cb1", findButton(t, bs, "Отправить").Payload))
	txt, bs := e.max.last("cb1")
	if !strings.Contains(txt, "Где именно") {
		t.Fatalf("place question = %q", txt)
	}
	e.handle(t, press(8801, "cb2", findButton(t, bs, "Подъезд 2, лестничная клетка").Payload))
	if txt, bs := e.max.last("cb2"); !strings.Contains(txt, "отправлена") || findButton(t, bs, "Оставить телефон для мастера").Type != "request_contact" {
		t.Fatalf("after place = %q %+v", txt, bs)
	}
	is, err := e.issues.Get(t.Context(), "i-1")
	if err != nil || is.ObjectID() != "h-1-e2-light" || is.Description() != "Не горит свет на лестнице" {
		t.Fatalf("issue = %+v, err = %v", is, err)
	}
}

// Телефон для мастера из кнопки контакта: подпись MAX проверяется, пересланный контакт без неё не подходит.
func TestContactSharesPhone(t *testing.T) {
	e := newEnv(t)
	u := e.resident(8901, true)
	vcf := "BEGIN:VCARD\r\nVERSION:3.0\r\nTEL;TYPE=cell:79990000123\r\nEND:VCARD\r\n"
	mac := hmac.New(sha256.New, []byte(testBotToken))
	mac.Write([]byte(vcf))
	contact := func(hash string) maxapi.Update {
		m := text(8901, "")
		payload, _ := json.Marshal(map[string]string{"vcf_info": vcf, "hash": hash})
		m.Message.Body.Attachments = []maxapi.IncomingAttachment{{Type: "contact", Payload: jsontext.Value(payload)}}
		return m
	}
	e.handle(t, contact(""))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Не получилось подтвердить") {
		t.Fatalf("contact without hash = %q", txt)
	}
	e.handle(t, contact(hex.EncodeToString(mac.Sum(nil))))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "Телефон сохранён") {
		t.Fatalf("contact = %q", txt)
	}
	if saved, _ := e.store.Users().Get(t.Context(), u.ID); saved.Phone != "+79990000123" {
		t.Fatalf("phone = %q", saved.Phone)
	}
}

// Совет дома в чате: житель пишет предложение, председатель получает его в очередь уведомлений
// и берёт в работу, автор получает ответ; голос в опросе кнопкой сменяется итогами.
func TestCouncilInChat(t *testing.T) {
	e := newEnv(t)
	anna := e.resident(9001, true)
	nina := user.User{ID: 9002, MaxUserID: 9002, Role: user.RoleResident, HouseID: "h-1", ChairmanHouseID: "h-1", ConsentVersion: "v1"}
	e.store.AddUser(nina)

	e.handle(t, text(9001, "/polls"))
	txt, bs := e.max.last("")
	if !strings.Contains(txt, "Совет дома") || !strings.Contains(txt, "Открытых опросов сейчас нет") {
		t.Fatalf("/polls = %q", txt)
	}
	e.handle(t, press(9001, "cb1", findButton(t, bs, "Предложить совету").Payload))
	e.handle(t, text(9001, "Поставить лавочку у второго подъезда"))
	if txt, _ := e.max.last(""); !strings.Contains(txt, "отправлено председателю") {
		t.Fatalf("after proposal = %q", txt)
	}
	queued := e.store.Outbox().(*apptest.MemOutbox).Pending
	if len(queued) != 1 || queued[0].Kind != app.NotifyProposal || queued[0].UserID != nina.ID {
		t.Fatalf("queued = %+v", queued)
	}

	e.handle(t, text(9002, "/polls"))
	_, bs = e.max.last("")
	e.handle(t, press(9002, "cb2", findButton(t, bs, "Папка предложений").Payload))
	prop, pbs := e.max.last("")
	if !strings.Contains(prop, "лавочку") || strings.Contains(prop, "Анна") {
		t.Fatalf("folder item = %q", prop)
	}
	e.handle(t, press(9002, "cb3", findButton(t, pbs, "Взять в работу").Payload))
	if txt, _ := e.max.last("cb3"); !strings.Contains(txt, "взято в работу") {
		t.Fatalf("accept = %q", txt)
	}
	queued = e.store.Outbox().(*apptest.MemOutbox).Pending
	if last := queued[len(queued)-1]; last.Kind != app.NotifyProposalAnswer || last.UserID != anna.ID {
		t.Fatalf("answer notification = %+v", last)
	}

	if _, err := e.council.CreatePoll(t.Context(), nina, appcouncil.PollInput{Question: "Ставим лавочку?", Options: []string{"За", "Против"}, Days: 7}); err != nil {
		t.Fatal(err)
	}
	e.handle(t, text(9001, "/polls"))
	poll := e.max.sent[len(e.max.sent)-2].msg
	if !strings.Contains(poll.Text, "Ставим лавочку") {
		t.Fatalf("poll message = %q", poll.Text)
	}
	e.handle(t, press(9001, "cb4", findButton(t, buttons(poll), "За").Payload))
	if txt, _ := e.max.last("cb4"); !strings.Contains(txt, "За: 100%, ваш голос") || !strings.Contains(txt, "Всего 1 голос") {
		t.Fatalf("results = %q", txt)
	}
}

func TestRoleSwitch(t *testing.T) {
	e := newEnv(t)
	e.resident(9901, true)
	// Без аргументов показывает меню выбора ролей
	e.handle(t, text(9901, "/role"))
	txt, bs := e.max.last("")
	if !strings.Contains(txt, "Ваша текущая роль") || len(bs) != 4 {
		t.Fatalf("/role = %q %+v", txt, bs)
	}
	findButton(t, bs, "Стать председателем")
	findButton(t, bs, "Стать сотрудником УК")
	findButton(t, bs, "Управа района")

	// Переключение на председателя
	e.handle(t, text(9901, "/role chairman"))
	txt, _ = e.max.last("")
	if !strings.Contains(txt, "Председатель совета") {
		t.Fatalf("/role chairman = %q", txt)
	}
	u, _ := e.store.Users().ByMaxID(t.Context(), 9901)
	if u.ChairmanHouseID != "h-1" {
		t.Fatalf("chairman house id = %q, want h-1", u.ChairmanHouseID)
	}

	// Переключение на УК
	e.handle(t, text(9901, "/role uk"))
	txt, bs = e.max.last("")
	if !strings.Contains(txt, "Сотрудник управляющей компании") || len(bs) != 1 {
		t.Fatalf("/role uk = %q %+v", txt, bs)
	}
	u, _ = e.store.Users().ByMaxID(t.Context(), 9901)
	if u.Role != user.RoleOperator {
		t.Fatalf("role = %q, want operator", u.Role)
	}

	// Сброс на жителя
	e.handle(t, text(9901, "/role resident"))
	txt, _ = e.max.last("")
	if !strings.Contains(txt, "обычный житель") {
		t.Fatalf("/role resident = %q", txt)
	}
	u, _ = e.store.Users().ByMaxID(t.Context(), 9901)
	if u.Role != user.RoleResident || u.ChairmanHouseID != "" {
		t.Fatalf("role = %q, chairman = %q", u.Role, u.ChairmanHouseID)
	}
}
