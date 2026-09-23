package auth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"dommax/internal/app/auth"
)

const botToken = "test-bot-token"

var now = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// sign собирает initData так, как это делает MAX: значения закодированы, hash — по отсортированным парам.
func sign(t *testing.T, token string, params map[string]string) string {
	t.Helper()
	keys := slices.Sorted(maps.Keys(params))
	var lines, pairs []string
	for _, k := range keys {
		lines = append(lines, k+"="+params[k])
		pairs = append(pairs, k+"="+url.QueryEscape(params[k]))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	return strings.Join(pairs, "&") + "&hash=" + hex.EncodeToString(mac.Sum(nil))
}

func validParams() map[string]string {
	return map[string]string{
		"auth_date":   strconv.FormatInt(now.Add(-5*time.Minute).Unix(), 10),
		"query_id":    "4c0ab423-342b-4e45-aea4-2747dbc500cd",
		"user":        `{"id":67890,"first_name":"Анна","last_name":"","username":null,"language_code":"ru"}`,
		"start_param": "o_h-17k2-e2-lift",
	}
}

func TestParseInitDataAcceptsValidSignature(t *testing.T) {
	d, err := auth.ParseInitData(sign(t, botToken, validParams()), botToken, now)
	if err != nil {
		t.Fatalf("ParseInitData: %v", err)
	}
	if d.User.ID != 67890 || d.User.FirstName != "Анна" || d.StartParam != "o_h-17k2-e2-lift" {
		t.Fatalf("data = %+v", d)
	}
}

func TestParseInitDataRejectsWrongToken(t *testing.T) {
	_, err := auth.ParseInitData(sign(t, "other-token", validParams()), botToken, now)
	if !errors.Is(err, auth.ErrInvalidInitData) {
		t.Fatalf("err = %v, want ErrInvalidInitData", err)
	}
}

func TestParseInitDataRejectsTamperedValue(t *testing.T) {
	raw := strings.Replace(sign(t, botToken, validParams()), "67890", "11111", 1)
	if _, err := auth.ParseInitData(raw, botToken, now); !errors.Is(err, auth.ErrInvalidInitData) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseInitDataRejectsStaleAuthDate(t *testing.T) {
	p := validParams()
	p["auth_date"] = strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10)
	if _, err := auth.ParseInitData(sign(t, botToken, p), botToken, now); !errors.Is(err, auth.ErrInvalidInitData) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseInitDataRejectsDuplicateKeysAndMissingHash(t *testing.T) {
	raw := sign(t, botToken, validParams())
	if _, err := auth.ParseInitData(raw+"&user=x", botToken, now); !errors.Is(err, auth.ErrInvalidInitData) {
		t.Fatalf("duplicate key err = %v", err)
	}
	noHash, _, _ := strings.Cut(raw, "&hash=")
	if _, err := auth.ParseInitData(noHash, botToken, now); !errors.Is(err, auth.ErrInvalidInitData) {
		t.Fatalf("missing hash err = %v", err)
	}
}
