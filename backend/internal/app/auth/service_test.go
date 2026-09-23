package auth_test

import (
	"errors"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/auth"
	"dommax/internal/domain/house"
	"dommax/internal/domain/user"
)

func newService(t *testing.T, demo bool) (*auth.Service, *apptest.MemStore) {
	t.Helper()
	s := apptest.New()
	// В MemStore demo-ключ ищется по FirstName.
	s.AddUser(user.User{ID: 1, FirstName: "resident_demo_1", Role: user.RoleResident})
	s.AddUser(user.User{ID: 3, FirstName: "uk_operator_demo", Role: user.RoleOperator, OrganizationID: "org-1"})
	clock := now
	svc := auth.NewService(s, auth.Config{
		BotToken:      botToken,
		SessionSecret: "session-secret",
		SessionTTL:    12 * time.Hour,
		DemoEnabled:   demo,
		Now:           func() time.Time { return clock },
	})
	return svc, s
}

func TestLoginMaxCreatesResidentOnce(t *testing.T) {
	svc, _ := newService(t, false)
	raw := sign(t, botToken, validParams())

	first, err := svc.LoginMax(t.Context(), raw)
	if err != nil {
		t.Fatalf("LoginMax: %v", err)
	}
	if first.User.MaxUserID != 67890 || first.User.Role != user.RoleResident || first.Token == "" {
		t.Fatalf("session = %+v", first)
	}
	second, err := svc.LoginMax(t.Context(), raw)
	if err != nil || second.User.ID != first.User.ID {
		t.Fatalf("second login = %+v, err = %v", second.User, err)
	}
}

func TestSessionTokenAuthenticates(t *testing.T) {
	svc, _ := newService(t, true)
	s, err := svc.LoginDemo(t.Context(), "uk_operator")
	if err != nil {
		t.Fatalf("LoginDemo: %v", err)
	}
	u, err := svc.Authenticate(t.Context(), s.Token)
	if err != nil || u.ID != 3 || u.Role != user.RoleOperator {
		t.Fatalf("user = %+v, err = %v", u, err)
	}
	if _, err := svc.Authenticate(t.Context(), s.Token+"x"); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("tampered token err = %v", err)
	}
}

func TestSessionExpires(t *testing.T) {
	svc, s := newService(t, true)
	sess, _ := svc.LoginDemo(t.Context(), "resident")
	later := auth.NewService(s, auth.Config{
		SessionSecret: "session-secret", SessionTTL: 12 * time.Hour,
		Now: func() time.Time { return now.Add(13 * time.Hour) },
	})
	if _, err := later.Authenticate(t.Context(), sess.Token); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("expired token err = %v", err)
	}
}

func TestDemoLoginDisabledByDefault(t *testing.T) {
	svc, _ := newService(t, false)
	if _, err := svc.LoginDemo(t.Context(), "resident"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestDemoLoginUnknownRole(t *testing.T) {
	svc, _ := newService(t, true)
	if _, err := svc.LoginDemo(t.Context(), "admin"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestAccountConsentHouseAndDelete(t *testing.T) {
	svc, s := newService(t, true)
	s.HouseMap["h-1"] = house.House{ID: "h-1"}
	u := s.UserMap[1]

	u, err := svc.AcceptConsent(t.Context(), u, "v1")
	if err != nil || !u.HasConsent("v1") {
		t.Fatalf("consent: %+v, %v", u, err)
	}
	if _, err := svc.SetHouse(t.Context(), u, "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown house err = %v", err)
	}
	u, err = svc.SetHouse(t.Context(), u, "h-1")
	if err != nil || s.UserMap[1].HouseID != "h-1" {
		t.Fatalf("house: %+v, %v", s.UserMap[1], err)
	}
	if err := svc.DeleteAccount(t.Context(), u); err != nil || !s.UserMap[1].Deleted() {
		t.Fatalf("delete: %+v, %v", s.UserMap[1], err)
	}
}

func TestDeletedUserCannotAuthenticate(t *testing.T) {
	svc, s := newService(t, true)
	sess, _ := svc.LoginDemo(t.Context(), "resident")
	u := s.UserMap[1]
	u.Delete(now)
	s.AddUser(u)
	if _, err := svc.Authenticate(t.Context(), sess.Token); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("err = %v", err)
	}
}
