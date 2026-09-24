package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dommax/internal/domain/user"
)

var ErrInvalidContact = errors.New("auth: invalid contact")

// Contact — то, что мини-приложение получило от WebApp.requestContact().
type Contact struct {
	Phone    string
	AuthDate string
	Hash     string
}

// VerifyContact проверяет, что номер привязан к аккаунту MAX этого пользователя
// (dev-max/docs/webapps/bridge.md, requestContact): hash = HMAC_SHA256(data, botToken), где
// data — пары authDate, phone и userId по алфавиту через \n, а phone без «+».
// Кодировка хеша и единицы authDate в документации не указаны: хеш принимается в hex любого
// регистра, authDate — в секундах или миллисекундах (больше 1e12). Подпись не старше
// InitDataTTL. Возвращает нормализованный номер «+цифры».
func VerifyContact(c Contact, maxUserID int64, botToken string, now time.Time) (string, error) {
	if botToken == "" {
		return "", fmt.Errorf("%w: bot token is not configured", ErrInvalidContact)
	}
	if c.Hash == "" || c.Phone == "" || maxUserID == 0 {
		return "", fmt.Errorf("%w: phone, authDate and hash are required", ErrInvalidContact)
	}
	data := "authDate=" + c.AuthDate + "\nphone=" + strings.TrimPrefix(c.Phone, "+") + "\nuserId=" + strconv.FormatInt(maxUserID, 10)
	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte(data))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(strings.ToLower(c.Hash))) {
		return "", fmt.Errorf("%w: signature mismatch", ErrInvalidContact)
	}

	n, err := strconv.ParseInt(c.AuthDate, 10, 64)
	if err != nil {
		return "", fmt.Errorf("%w: authDate", ErrInvalidContact)
	}
	signed := time.Unix(n, 0)
	if n > 1e12 {
		signed = time.UnixMilli(n)
	}
	if age := now.Sub(signed); age > InitDataTTL || age < -time.Minute {
		return "", fmt.Errorf("%w: authDate is outside %s", ErrInvalidContact, InitDataTTL)
	}

	phone, err := user.NormalizePhone(c.Phone)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidContact, err)
	}
	return phone, nil
}
