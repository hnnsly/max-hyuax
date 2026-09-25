package auth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"dommax/internal/app/auth"
)

// Свой тестовый вектор: официального в документации нет. Подпись — HMAC-SHA256 от vcf_info с токеном бота.
func TestVerifyBotContact(t *testing.T) {
	const token = "bot-token"
	vcf := "BEGIN:VCARD\r\nVERSION:3.0\r\nPRODID:ez-vcard 0.10.3\r\nTEL;TYPE=cell:79990000000\r\nFN:Ivan Ivanov\r\nEND:VCARD\r\n"
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(vcf))
	sum := mac.Sum(nil)

	for name, hash := range map[string]string{
		"hex":        hex.EncodeToString(sum),
		"hex upper":  strings.ToUpper(hex.EncodeToString(sum)),
		"base64":     base64.StdEncoding.EncodeToString(sum),
		"base64 url": base64.RawURLEncoding.EncodeToString(sum),
	} {
		phone, err := auth.VerifyBotContact(vcf, hash, token)
		if err != nil || phone != "+79990000000" {
			t.Errorf("%s: phone = %q, err = %v", name, phone, err)
		}
	}
	// В JSON переносы могут прийти экранированными: подпись считается по настоящим.
	escaped := strings.ReplaceAll(vcf, "\r\n", `\r\n`)
	if phone, err := auth.VerifyBotContact(escaped, hex.EncodeToString(sum), token); err != nil || phone != "+79990000000" {
		t.Errorf("escaped vcf: phone = %q, err = %v", phone, err)
	}

	bad := []struct{ name, vcf, hash, token string }{
		{"other token", vcf, hex.EncodeToString(sum), "другой"},
		{"changed number", strings.Replace(vcf, "79990000000", "79991111111", 1), hex.EncodeToString(sum), token},
		{"empty hash", vcf, "", token},
		{"no bot token", vcf, hex.EncodeToString(sum), ""},
	}
	for _, b := range bad {
		if _, err := auth.VerifyBotContact(b.vcf, b.hash, b.token); !errors.Is(err, auth.ErrInvalidContact) {
			t.Errorf("%s: err = %v, want ErrInvalidContact", b.name, err)
		}
	}
}
