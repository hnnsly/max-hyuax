package user_test

import (
	"errors"
	"testing"

	"dommax/internal/domain/user"
)

func TestNormalizePhone(t *testing.T) {
	for in, want := range map[string]string{
		"+79991234567":       "+79991234567",
		"79991234567":        "+79991234567",
		"+7 (999) 123-45-67": "+79991234567",
		"+375291234567":      "+375291234567",
	} {
		got, err := user.NormalizePhone(in)
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "123", "abc", "+7999123456789012", "+7 999 12a 45 67"} {
		if _, err := user.NormalizePhone(bad); !errors.Is(err, user.ErrInvalidPhone) {
			t.Errorf("NormalizePhone(%q) err = %v, want ErrInvalidPhone", bad, err)
		}
	}
}

// Телефон, оставленный жителем, и есть его согласие показывать номер УК: убрать можно в любой момент.
func TestSharePhoneAndHide(t *testing.T) {
	u := user.User{ID: 1, Role: user.RoleResident}
	u.SharePhone("+79991234567")
	if !u.PhoneShared() || u.Phone != "+79991234567" {
		t.Fatalf("after share: %+v", u)
	}
	u.HidePhone()
	if u.PhoneShared() || u.Phone != "" {
		t.Fatalf("after hide: %+v", u)
	}
}
