package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"strings"
	"time"
)

var errBadToken = errors.New("auth: bad session token")

type claims struct {
	UserID  int64 `json:"u"`
	Expires int64 `json:"e"`
}

var b64 = base64.RawURLEncoding

// signToken выпускает токен вида base64(claims).base64(HMAC_SHA256(secret, claims)).
// Своя схема вместо JWT: одна подпись, один алгоритм, без внешних зависимостей.
func signToken(secret string, userID int64, exp time.Time) string {
	body, _ := json.Marshal(claims{UserID: userID, Expires: exp.Unix()})
	payload := b64.EncodeToString(body)
	return payload + "." + b64.EncodeToString(mac(secret, payload))
}

func verifyToken(secret, token string, now time.Time) (int64, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return 0, errBadToken
	}
	got, err := b64.DecodeString(sig)
	if err != nil || !hmac.Equal(got, mac(secret, payload)) {
		return 0, errBadToken
	}
	body, err := b64.DecodeString(payload)
	if err != nil {
		return 0, errBadToken
	}
	var c claims
	if err := json.Unmarshal(body, &c); err != nil || c.UserID == 0 {
		return 0, errBadToken
	}
	if !now.Before(time.Unix(c.Expires, 0)) {
		return 0, errBadToken
	}
	return c.UserID, nil
}

func mac(secret, payload string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return h.Sum(nil)
}
