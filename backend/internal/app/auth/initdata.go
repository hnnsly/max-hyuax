// Пакет auth — вход в мини-приложение по initData MAX и демо-вход, собственные сессии (ADR-004).
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidInitData = errors.New("auth: invalid initData")

// InitDataTTL — максимальный возраст auth_date, рекомендованный документацией MAX.
const InitDataTTL = time.Hour

type MaxUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type InitData struct {
	User       MaxUser
	AuthDate   time.Time
	StartParam string
}

// ParseInitData проверяет подпись initData по алгоритму MAX (dev-max/docs/webapps/validation.md):
// secret = HMAC_SHA256("WebAppData", token), hash = hex(HMAC_SHA256(secret, launch_params)),
// где launch_params — декодированные пары key=value без hash, отсортированные по ключу и
// соединённые через \n. Каждый ключ встречается один раз, auth_date не старше InitDataTTL.
func ParseInitData(raw, botToken string, now time.Time) (InitData, error) {
	if botToken == "" {
		return InitData{}, fmt.Errorf("%w: bot token is not configured", ErrInvalidInitData)
	}
	var (
		hash  string
		lines []string
		vals  = map[string]string{}
	)
	for pair := range strings.SplitSeq(raw, "&") {
		key, val, ok := strings.Cut(pair, "=")
		if !ok {
			return InitData{}, fmt.Errorf("%w: malformed pair", ErrInvalidInitData)
		}
		if _, dup := vals[key]; dup || (key == "hash" && hash != "") {
			return InitData{}, fmt.Errorf("%w: duplicate key %q", ErrInvalidInitData, key)
		}
		decoded, err := url.QueryUnescape(val)
		if err != nil {
			return InitData{}, fmt.Errorf("%w: %v", ErrInvalidInitData, err)
		}
		if key == "hash" {
			hash = decoded
			continue
		}
		vals[key] = decoded
		lines = append(lines, key+"="+decoded)
	}
	if hash == "" {
		return InitData{}, fmt.Errorf("%w: hash is missing", ErrInvalidInitData)
	}
	slices.Sort(lines)

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(hash)) {
		return InitData{}, fmt.Errorf("%w: signature mismatch", ErrInvalidInitData)
	}

	sec, err := strconv.ParseInt(vals["auth_date"], 10, 64)
	if err != nil {
		return InitData{}, fmt.Errorf("%w: auth_date", ErrInvalidInitData)
	}
	authDate := time.Unix(sec, 0)
	if age := now.Sub(authDate); age > InitDataTTL || age < -time.Minute {
		return InitData{}, fmt.Errorf("%w: auth_date is outside %s", ErrInvalidInitData, InitDataTTL)
	}

	d := InitData{AuthDate: authDate, StartParam: vals["start_param"]}
	if err := json.Unmarshal([]byte(vals["user"]), &d.User); err != nil || d.User.ID == 0 {
		return InitData{}, fmt.Errorf("%w: user", ErrInvalidInitData)
	}
	return d, nil
}
