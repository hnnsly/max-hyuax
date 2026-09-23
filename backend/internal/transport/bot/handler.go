// Пакет bot превращает события MAX в ответы бота. Long polling и webhook
// передают события в один и тот же Handler.
package bot

import (
	"context"
	"log/slog"

	"dommax/internal/storage/maxapi"
)

// Payload callback-кнопок.
const PayloadReport = "report"

// Messenger — часть Bot API, которая нужна обработчику.
type Messenger interface {
	Send(ctx context.Context, to maxapi.Target, m maxapi.NewMessage) (maxapi.Message, error)
	Answer(ctx context.Context, callbackID string, a maxapi.CallbackAnswer) error
}

type Handler struct {
	max     Messenger
	botName string
	log     *slog.Logger
}

// NewHandler принимает username бота: кнопки open_app запускают мини-приложение этого бота.
func NewHandler(m Messenger, botName string, log *slog.Logger) *Handler {
	return &Handler{max: m, botName: botName, log: log}
}

const greetingText = "Здравствуйте! Я помогаю соседям сообщать о поломках в доме: лифт, свет в подъезде, протечка, отопление.\n\n" +
	"Одна заявка на весь дом вместо десятка сообщений в чате. Я покажу, кто отвечает и до какого срока, и напишу, когда статус изменится."

func (h *Handler) Handle(ctx context.Context, u maxapi.Update) error {
	switch u.Type {
	case maxapi.UpdateBotStarted:
		return h.greet(ctx, target(u.ChatID, u.User.UserID), u.Payload)
	case maxapi.UpdateMessageCreated:
		return h.onMessage(ctx, u.Message)
	case maxapi.UpdateMessageCallback:
		return h.onCallback(ctx, u.Callback)
	}
	return nil
}

func (h *Handler) onMessage(ctx context.Context, m *maxapi.Message) error {
	// Бот отвечает только в личных диалогах и не пишет в домовые чаты (требования MAX §1.5).
	if m == nil || m.Sender.IsBot || m.Recipient.ChatType != "dialog" {
		return nil
	}
	// Пока нет диалога заявки, на /start и любой другой текст бот отвечает приветствием с кнопками входа.
	return h.greet(ctx, target(m.Recipient.ChatID, m.Sender.UserID), "")
}

func (h *Handler) onCallback(ctx context.Context, cb *maxapi.Callback) error {
	if cb == nil {
		return nil
	}
	switch cb.Payload {
	case PayloadReport:
		return h.max.Answer(ctx, cb.ID, maxapi.CallbackAnswer{Notification: "Опишите проблему одним сообщением: что сломалось и где."})
	}
	return nil
}

func (h *Handler) greet(ctx context.Context, to maxapi.Target, startPayload string) error {
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{
		Text: greetingText,
		Attachments: []maxapi.Attachment{maxapi.Keyboard(
			[]maxapi.Button{maxapi.CallbackButton("Сообщить о проблеме", PayloadReport)},
			[]maxapi.Button{maxapi.OpenAppButton("Открыть приложение", h.botName, startPayload)},
		)},
	})
	return err
}

func target(chatID, userID int64) maxapi.Target {
	if chatID != 0 {
		return maxapi.ToChat(chatID)
	}
	return maxapi.ToUser(userID)
}
