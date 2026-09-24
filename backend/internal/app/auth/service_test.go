package auth_test

import (
	"errors"
	"strconv"
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
	s.AddUser(user.User{ID: 1, FirstName: "Анна", Role: user.RoleResident, ConsentVersion: "v1", ConsentAt: now})
	s.AddUser(user.User{ID: 3, FirstName: "Оператор УК", Role: user.RoleOperator, OrganizationID: "org-1"})
	s.DemoKeys = map[string]int64{"resident_demo_1": 1, "uk_operator_demo": 3}
	clock := now
	svc := auth.NewService(s, auth.Config{
		BotToken:       botToken,
		SessionSecret:  "session-secret",
		SessionTTL:     12 * time.Hour,
		DemoEnabled:    demo,
		ConsentVersion: "v1",
		Now:            func() time.Time { return clock },
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

func TestEnsureMaxUserFindsOrCreates(t *testing.T) {
	svc, _ := newService(t, false)
	u1, err := svc.EnsureMaxUser(t.Context(), 777, "Ольга")
	if err != nil || u1.MaxUserID != 777 || u1.Role != user.RoleResident {
		t.Fatalf("u1 = %+v, err = %v", u1, err)
	}
	u2, err := svc.EnsureMaxUser(t.Context(), 777, "Ольга")
	if err != nil || u2.ID != u1.ID {
		t.Fatalf("u2 = %+v, err = %v", u2, err)
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
	if err := svc.DeleteAccount(t.Context(), u); err != nil || !s.UserMap[1].Deleted() || s.UserMap[1].HouseID != "" {
		t.Fatalf("delete: %+v, %v", s.UserMap[1], err)
	}
}

// Житель MAX удалил аккаунт и вернулся: это новый аккаунт, старые заявки к нему не привязаны.
func TestReturningMaxUserStartsFresh(t *testing.T) {
	svc, s := newService(t, true)
	first, err := svc.EnsureMaxUser(t.Context(), 777, "Ольга")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteAccount(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	if s.UserMap[first.ID].MaxUserID != 0 {
		t.Fatalf("deleted user keeps MAX id: %+v", s.UserMap[first.ID])
	}
	again, err := svc.EnsureMaxUser(t.Context(), 777, "Ольга")
	if err != nil || again.ID == first.ID || again.HasConsent("v1") || again.Deleted() {
		t.Fatalf("again = %+v, err = %v, want a new account without consent", again, err)
	}
}

// Телефон оставляет только пользователь MAX с подписью от клиента; убрать его можно в любой момент.
func TestSharePhoneRequiresSignedContact(t *testing.T) {
	svc, s := newService(t, true)
	u, err := svc.EnsureMaxUser(t.Context(), 67890, "Ольга")
	if err != nil {
		t.Fatal(err)
	}
	sec := strconv.FormatInt(now.Unix(), 10)
	good := auth.Contact{Phone: "+79991234567", AuthDate: sec, Hash: signContact(botToken, sec, "+79991234567", 67890)}

	if _, err := svc.SharePhone(t.Context(), u, auth.Contact{Phone: "+79991234567", AuthDate: sec, Hash: "00"}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("bad hash err = %v, want ErrInvalidInput", err)
	}
	got, err := svc.SharePhone(t.Context(), u, good)
	if err != nil || !got.PhoneShared() || s.UserMap[u.ID].Phone != "+79991234567" {
		t.Fatalf("share: %+v, err = %v", s.UserMap[u.ID], err)
	}
	got, err = svc.HidePhone(t.Context(), got)
	if err != nil || got.PhoneShared() || s.UserMap[u.ID].Phone != "" {
		t.Fatalf("hide: %+v, err = %v", s.UserMap[u.ID], err)
	}
	// Демо-вход без MAX: подписи номера быть не может.
	if _, err := svc.SharePhone(t.Context(), s.UserMap[1], good); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("demo user err = %v, want ErrForbidden", err)
	}
}

// Проверяющий нажал «Не показывать телефон» у демо-Анны: следующий демо-вход возвращает
// синтетический номер, иначе блок «Контакты жителей» пропадёт из демо для всех.
func TestDemoLoginRestoresDemoPhone(t *testing.T) {
	svc, s := newService(t, true)
	sess, err := svc.LoginDemo(t.Context(), "resident")
	if err != nil || sess.User.Phone != "+79990000001" {
		t.Fatalf("first login phone = %q, err = %v", sess.User.Phone, err)
	}
	if _, err := svc.HidePhone(t.Context(), sess.User); err != nil {
		t.Fatal(err)
	}
	sess, err = svc.LoginDemo(t.Context(), "resident")
	if err != nil || sess.User.Phone != "+79990000001" || s.UserMap[1].Phone != "+79990000001" {
		t.Fatalf("after hide: phone = %q, stored %q, err = %v", sess.User.Phone, s.UserMap[1].Phone, err)
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

// Проверяющий удалил демо-аккаунт: следующий демо-вход возвращает его в исходное состояние,
// иначе демо и проверки DATA-API сломаются для всех.
func TestDemoLoginRestoresDeletedDemoUser(t *testing.T) {
	svc, s := newService(t, true)
	if err := svc.DeleteAccount(t.Context(), s.UserMap[1]); err != nil {
		t.Fatal(err)
	}
	sess, err := svc.LoginDemo(t.Context(), "resident")
	if err != nil {
		t.Fatalf("LoginDemo after delete: %v", err)
	}
	// Синтетический телефон демо-жительницы тоже возвращается: без него УК в демо не увидит контактов.
	if u := sess.User; u.Deleted() || u.FirstName != "Анна" || !u.HasConsent("v1") || u.HouseID != "h-17k2" || u.Phone != "+79990000001" {
		t.Fatalf("restored user = %+v, want the demo user as seeded", u)
	}
	if _, err := svc.Authenticate(t.Context(), sess.Token); err != nil {
		t.Fatalf("Authenticate restored demo user: %v", err)
	}
}
