package maxapi_test

import (
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dommax/internal/storage/maxapi"
)

const token = "test-token"

func newServer(t *testing.T, h http.HandlerFunc) *maxapi.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != token {
			t.Errorf("Authorization = %q, want raw token", got)
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return maxapi.New(srv.URL, token, srv.Client())
}

func TestUpdatesDecodesEventsAndPassesMarker(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/updates" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("marker") != "41" || q.Get("timeout") != "25" {
			t.Errorf("query = %v", q)
		}
		io.WriteString(w, `{"marker":42,"updates":[
			{"update_type":"bot_started","timestamp":1,"chat_id":7,"user":{"user_id":1001,"first_name":"Анна"},"payload":"o_lift_e2"},
			{"update_type":"message_created","timestamp":2,"message":{"sender":{"user_id":1001},"recipient":{"chat_id":7,"chat_type":"dialog"},"body":{"mid":"m1","text":"/start"}}},
			{"update_type":"message_callback","timestamp":3,"callback":{"callback_id":"cb1","payload":"report","user":{"user_id":1001}},"message":{"recipient":{"chat_id":7},"body":{"mid":"m2"}}}
		]}`)
	})

	page, err := c.Updates(t.Context(), 41, 25*time.Second)
	if err != nil {
		t.Fatalf("Updates: %v", err)
	}
	if page.Marker != 42 || len(page.Updates) != 3 {
		t.Fatalf("page = %+v", page)
	}
	started, msg, cb := page.Updates[0], page.Updates[1], page.Updates[2]
	if started.Type != maxapi.UpdateBotStarted || started.ChatID != 7 || started.User.UserID != 1001 || started.Payload != "o_lift_e2" {
		t.Errorf("bot_started = %+v", started)
	}
	if msg.Type != maxapi.UpdateMessageCreated || msg.Message.Body.Text != "/start" || msg.Message.Recipient.ChatID != 7 {
		t.Errorf("message_created = %+v", msg)
	}
	if cb.Type != maxapi.UpdateMessageCallback || cb.Callback.ID != "cb1" || cb.Callback.Payload != "report" || cb.Callback.User.UserID != 1001 {
		t.Errorf("message_callback = %+v", cb)
	}
}

func TestUpdatesWithoutMarkerOmitsIt(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("marker") {
			t.Errorf("marker must be omitted, query = %v", r.URL.Query())
		}
		io.WriteString(w, `{"updates":[],"marker":null}`)
	})
	if _, err := c.Updates(t.Context(), 0, 0); err != nil {
		t.Fatalf("Updates: %v", err)
	}
}

func TestSendPostsMessageWithKeyboardToChat(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/messages" || r.URL.Query().Get("chat_id") != "7" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["text"] != "Здравствуйте" {
			t.Errorf("text = %v", body["text"])
		}
		att := body["attachments"].([]any)[0].(map[string]any)
		btn := att["payload"].(map[string]any)["buttons"].([]any)[0].([]any)[0].(map[string]any)
		if att["type"] != "inline_keyboard" || btn["type"] != "callback" || btn["payload"] != "report" {
			t.Errorf("attachment = %v", att)
		}
		if _, has := btn["url"]; has {
			t.Errorf("empty fields must be omitted: %v", btn)
		}
		io.WriteString(w, `{"message":{"recipient":{"chat_id":7},"body":{"mid":"m9","text":"Здравствуйте"}}}`)
	})

	msg, err := c.Send(t.Context(), maxapi.ToChat(7), maxapi.NewMessage{
		Text:        "Здравствуйте",
		Attachments: []maxapi.Attachment{maxapi.Keyboard([]maxapi.Button{maxapi.CallbackButton("Сообщить о проблеме", "report")})},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if msg.Body.MID != "m9" {
		t.Fatalf("mid = %q", msg.Body.MID)
	}
}

func TestSendWithoutAttachmentsOmitsField(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		// Пустой массив удалил бы вложения при редактировании, поэтому nil не отправляется.
		if _, has := body["attachments"]; has {
			t.Errorf("attachments must be omitted: %v", body)
		}
		io.WriteString(w, `{"message":{"body":{"mid":"m1"}}}`)
	})
	if _, err := c.Send(t.Context(), maxapi.ToUser(1001), maxapi.NewMessage{Text: "Привет"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestEditPutsMessageByID(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/messages" || r.URL.Query().Get("message_id") != "m9" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		io.WriteString(w, `{"success":true}`)
	})
	if err := c.Edit(t.Context(), "m9", maxapi.NewMessage{Text: "В работе"}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
}

func TestSubscribePostsURLTypesAndSecret(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/subscriptions" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Fatal(err)
		}
		if body["url"] != "https://dom.example/webhook/max" || body["secret"] != "s3cret" || len(body["update_types"].([]any)) != 3 {
			t.Errorf("body = %v", body)
		}
		io.WriteString(w, `{"success":true}`)
	})
	err := c.Subscribe(t.Context(), "https://dom.example/webhook/max", "s3cret",
		[]maxapi.UpdateType{maxapi.UpdateBotStarted, maxapi.UpdateMessageCreated, maxapi.UpdateMessageCallback})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
}

func TestSetCommandsPatchesMe(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/me/commands" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		var body struct {
			Commands []map[string]string `json:"commands"`
		}
		if err := json.UnmarshalRead(r.Body, &body); err != nil || len(body.Commands) != 1 || body.Commands[0]["name"] != "new" {
			t.Errorf("body = %+v, err = %v", body, err)
		}
		io.WriteString(w, `{"commands":[{"name":"new","description":"Сообщить о проблеме"}]}`)
	})
	if err := c.SetCommands(t.Context(), []maxapi.Command{{Name: "new", Description: "Сообщить о проблеме"}}); err != nil {
		t.Fatalf("SetCommands: %v", err)
	}
}

func TestAnswerSendsCallbackID(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/answers" || r.URL.Query().Get("callback_id") != "cb1" {
			t.Errorf("request = %s", r.URL)
		}
		io.WriteString(w, `{"success":true}`)
	})
	if err := c.Answer(t.Context(), "cb1", maxapi.CallbackAnswer{Notification: "Готово"}); err != nil {
		t.Fatalf("Answer: %v", err)
	}
}

func TestAnswerUnsuccessfulIsError(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":false,"message":"callback expired"}`)
	})
	if err := c.Answer(t.Context(), "cb1", maxapi.CallbackAnswer{Notification: "x"}); err == nil {
		t.Fatal("want error for success=false")
	}
}

func TestAPIErrorCarriesStatusAndMessage(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"code":"verify.token","message":"Invalid access_token"}`)
	})
	_, err := c.Me(t.Context())
	apiErr, ok := errors.AsType[*maxapi.Error](err)
	if !ok {
		t.Fatalf("err = %v, want *maxapi.Error", err)
	}
	if apiErr.Status != http.StatusUnauthorized || apiErr.Message != "Invalid access_token" || apiErr.RateLimited() {
		t.Fatalf("apiErr = %+v", apiErr)
	}
	if !(&maxapi.Error{Status: http.StatusTooManyRequests}).RateLimited() {
		t.Fatal("429 must be reported as rate limited")
	}
}

func TestMeDecodesBot(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"user_id":426323711,"first_name":"Хакатон МАХ 105","is_bot":true,"username":"t105_hakaton_max_bot"}`)
	})
	me, err := c.Me(t.Context())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.UserID != 426323711 || me.Username != "t105_hakaton_max_bot" || !me.IsBot {
		t.Fatalf("me = %+v", me)
	}
}
