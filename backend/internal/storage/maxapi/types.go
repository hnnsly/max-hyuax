package maxapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

type UpdateType string

const (
	UpdateBotStarted      UpdateType = "bot_started"
	UpdateMessageCreated  UpdateType = "message_created"
	UpdateMessageCallback UpdateType = "message_callback"
)

type User struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	IsBot     bool   `json:"is_bot"`
}

// Update — одно событие из long polling или webhook. Поля заполняются по Type:
// bot_started несёт ChatID, User и Payload (значение ?start=), message_created — Message,
// message_callback — Callback и Message.
type Update struct {
	Type      UpdateType `json:"update_type"`
	Timestamp int64      `json:"timestamp"`
	ChatID    int64      `json:"chat_id"`
	User      User       `json:"user"`
	Payload   string     `json:"payload"`
	Message   *Message   `json:"message"`
	Callback  *Callback  `json:"callback"`
}

type Callback struct {
	ID        string `json:"callback_id"`
	Payload   string `json:"payload"`
	User      User   `json:"user"`
	Timestamp int64  `json:"timestamp"`
}

type Message struct {
	Sender    User        `json:"sender"`
	Recipient Recipient   `json:"recipient"`
	Timestamp int64       `json:"timestamp"`
	Body      MessageBody `json:"body"`
}

type Recipient struct {
	ChatID   int64  `json:"chat_id"`
	ChatType string `json:"chat_type"`
	UserID   int64  `json:"user_id"`
}

type MessageBody struct {
	MID         string               `json:"mid"`
	Text        string               `json:"text"`
	Attachments []IncomingAttachment `json:"attachments"`
}

// IncomingAttachment хранит payload как есть: пока разбирается только геолокация.
type IncomingAttachment struct {
	Type      string         `json:"type"`
	Latitude  float64        `json:"latitude"`
	Longitude float64        `json:"longitude"`
	Payload   jsontext.Value `json:"payload"`
}

type UpdatesPage struct {
	Updates []Update `json:"updates"`
	Marker  int64    `json:"marker"`
}

// NewMessage — тело POST и PUT /messages. Пустые Attachments не отправляются:
// явный пустой массив удалил бы клавиатуру у редактируемого сообщения.
type NewMessage struct {
	Text        string       `json:"text,omitempty"`
	Attachments []Attachment `json:"attachments,omitzero"`
	Format      string       `json:"format,omitempty"`
	Notify      *bool        `json:"notify,omitzero"`
}

type Attachment struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type KeyboardPayload struct {
	Buttons [][]Button `json:"buttons"`
}

type Button struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Payload string `json:"payload,omitempty"`
	URL     string `json:"url,omitempty"`
	WebApp  string `json:"web_app,omitempty"`
	Quick   bool   `json:"quick,omitzero"`
}

// Keyboard собирает inline-клавиатуру: один аргумент — один ряд кнопок.
func Keyboard(rows ...[]Button) Attachment {
	return Attachment{Type: "inline_keyboard", Payload: KeyboardPayload{Buttons: rows}}
}

func CallbackButton(text, payload string) Button {
	return Button{Type: "callback", Text: text, Payload: payload}
}

func LinkButton(text, url string) Button { return Button{Type: "link", Text: text, URL: url} }

// OpenAppButton открывает мини-приложение бота; payload попадает в initData как start_param.
func OpenAppButton(text, bot, payload string) Button {
	return Button{Type: "open_app", Text: text, WebApp: bot, Payload: payload}
}

func GeoButton(text string) Button { return Button{Type: "request_geo_location", Text: text} }

type CallbackAnswer struct {
	Message      *NewMessage `json:"message,omitzero"`
	Notification string      `json:"notification,omitempty"`
}

// Target — адресат сообщения: диалог с пользователем или чат.
type Target struct {
	userID, chatID int64
}

func ToUser(id int64) Target { return Target{userID: id} }
func ToChat(id int64) Target { return Target{chatID: id} }

// PhotoURL — ссылка на присланное фото (вложение image). Срок жизни ссылки ограничен,
// и MAX отдаёт её не во всех клиентах: пустая строка, если ссылки нет.
func (a IncomingAttachment) PhotoURL() string {
	if a.Type != "image" || len(a.Payload) == 0 {
		return ""
	}
	var p struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(a.Payload, &p) != nil {
		return ""
	}
	return p.URL
}
