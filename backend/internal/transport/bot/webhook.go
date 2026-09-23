package bot

import (
	"context"
	"crypto/subtle"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"dommax/internal/storage/maxapi"
)

var (
	ErrBadSecret = errors.New("bot: webhook secret mismatch")
	ErrBadUpdate = errors.New("bot: malformed update")
)

// Deduper запоминает обработанные события: MAX может доставить одно событие повторно.
type Deduper interface {
	MarkUpdateProcessed(ctx context.Context, key string) (bool, error)
}

// handleTimeout ограничивает обработку одного события в фоне.
const handleTimeout = 30 * time.Second

// Webhook принимает события MAX. HTTP-ответ отдаётся сразу, обработка идёт в фоне:
// MAX ждёт 200 не дольше 30 секунд.
type Webhook struct {
	secret string
	h      UpdateHandler
	dedup  Deduper
	log    *slog.Logger
	wg     sync.WaitGroup
}

func NewWebhook(secret string, h UpdateHandler, dedup Deduper, log *slog.Logger) *Webhook {
	return &Webhook{secret: secret, h: h, dedup: dedup, log: log}
}

// Accept проверяет секрет из X-Max-Bot-Api-Secret, отбрасывает повторы и запускает обработку.
func (w *Webhook) Accept(ctx context.Context, secretHeader string, body []byte) error {
	if subtle.ConstantTimeCompare([]byte(secretHeader), []byte(w.secret)) != 1 {
		return ErrBadSecret
	}
	var u maxapi.Update
	if err := json.Unmarshal(body, &u); err != nil || u.Type == "" {
		return ErrBadUpdate
	}
	first, err := w.dedup.MarkUpdateProcessed(ctx, UpdateKey(u))
	if err != nil {
		return err
	}
	if !first {
		return nil
	}
	w.wg.Go(func() {
		hctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), handleTimeout)
		defer cancel()
		if err := w.h.Handle(hctx, u); err != nil {
			w.log.ErrorContext(hctx, "handle webhook update failed", "type", u.Type, "err", err)
		}
	})
	return nil
}

// Wait дожидается фоновой обработки; вызывается при остановке сервиса.
func (w *Webhook) Wait() { w.wg.Wait() }

// UpdateKey — ключ дедупликации: тип, время и самый точный идентификатор события.
func UpdateKey(u maxapi.Update) string {
	id := fmt.Sprintf("%d:%d", u.ChatID, u.User.UserID)
	switch {
	case u.Callback != nil:
		id = u.Callback.ID
	case u.Message != nil && u.Message.Body.MID != "":
		id = u.Message.Body.MID
	}
	return fmt.Sprintf("%s:%d:%s", u.Type, u.Timestamp, id)
}
