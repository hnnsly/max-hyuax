package user

import (
	"errors"
	"strings"
)

var ErrInvalidPhone = errors.New("user: invalid phone number")

// NormalizePhone приводит номер к виду «+цифры»: пробелы, скобки и дефисы убираются,
// цифр должно быть от 10 до 15 (E.164). Буквы и прочие символы — ошибка.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for i, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == '+' && i == 0, r == ' ', r == '-', r == '(', r == ')':
		default:
			return "", ErrInvalidPhone
		}
	}
	if n := digits.Len(); n < 10 || n > 15 {
		return "", ErrInvalidPhone
	}
	return "+" + digits.String(), nil
}

// SharePhone — житель оставил телефон для мастера: номер и есть согласие показывать его УК
// по заявкам, где он участник. Номер должен быть уже проверен и нормализован.
func (u *User) SharePhone(phone string) { u.Phone = phone }

// HidePhone — житель больше не показывает телефон УК.
func (u *User) HidePhone() { u.Phone = "" }

// PhoneShared сообщает, что УК может видеть телефон жителя по его заявкам.
func (u User) PhoneShared() bool { return u.Phone != "" }
