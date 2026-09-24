package auth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"dommax/internal/app/auth"
)

// signContact подписывает номер так, как описано в dev-max/docs/webapps/bridge.md (requestContact):
// HMAC_SHA256("authDate=…\nphone=…\nuserId=…", botToken), телефон без «+». Официального вектора
// в документации нет, поэтому подпись строится по описанию.
func signContact(token, authDate, phone string, userID int64) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte("authDate=" + authDate + "\nphone=" + strings.TrimPrefix(phone, "+") + "\nuserId=" + strconv.FormatInt(userID, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyContact(t *testing.T) {
	const userID = 67890
	sec := strconv.FormatInt(now.Add(-5*time.Minute).Unix(), 10)
	ms := strconv.FormatInt(now.Add(-5*time.Minute).UnixMilli(), 10)

	for name, c := range map[string]auth.Contact{
		"секунды, номер с плюсом": {Phone: "+79991234567", AuthDate: sec, Hash: signContact(botToken, sec, "+79991234567", userID)},
		"миллисекунды":            {Phone: "79991234567", AuthDate: ms, Hash: signContact(botToken, ms, "79991234567", userID)},
		"хеш в верхнем регистре":  {Phone: "+79991234567", AuthDate: sec, Hash: strings.ToUpper(signContact(botToken, sec, "+79991234567", userID))},
	} {
		phone, err := auth.VerifyContact(c, userID, botToken, now)
		if err != nil || phone != "+79991234567" {
			t.Errorf("%s: phone = %q, err = %v", name, phone, err)
		}
	}
}

func TestVerifyContactRejects(t *testing.T) {
	const userID = 67890
	sec := strconv.FormatInt(now.Add(-5*time.Minute).Unix(), 10)
	old := strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10)
	future := strconv.FormatInt(now.Add(10*time.Minute).Unix(), 10)
	good := signContact(botToken, sec, "+79991234567", userID)

	for name, tc := range map[string]struct {
		c     auth.Contact
		user  int64
		token string
	}{
		"чужой токен бота":       {auth.Contact{Phone: "+79991234567", AuthDate: sec, Hash: signContact("other", sec, "+79991234567", userID)}, userID, botToken},
		"подменённый номер":      {auth.Contact{Phone: "+79990000000", AuthDate: sec, Hash: good}, userID, botToken},
		"чужой пользователь":     {auth.Contact{Phone: "+79991234567", AuthDate: sec, Hash: good}, 11111, botToken},
		"старая подпись":         {auth.Contact{Phone: "+79991234567", AuthDate: old, Hash: signContact(botToken, old, "+79991234567", userID)}, userID, botToken},
		"подпись из будущего":    {auth.Contact{Phone: "+79991234567", AuthDate: future, Hash: signContact(botToken, future, "+79991234567", userID)}, userID, botToken},
		"нет подписи":            {auth.Contact{Phone: "+79991234567", AuthDate: sec}, userID, botToken},
		"дата не число":          {auth.Contact{Phone: "+79991234567", AuthDate: "вчера", Hash: good}, userID, botToken},
		"токен бота не настроен": {auth.Contact{Phone: "+79991234567", AuthDate: sec, Hash: good}, userID, ""},
		"не номер":               {auth.Contact{Phone: "abc", AuthDate: sec, Hash: signContact(botToken, sec, "abc", userID)}, userID, botToken},
	} {
		if _, err := auth.VerifyContact(tc.c, tc.user, tc.token, now); !errors.Is(err, auth.ErrInvalidContact) {
			t.Errorf("%s: err = %v, want ErrInvalidContact", name, err)
		}
	}
}
