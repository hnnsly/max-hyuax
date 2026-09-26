package appeal_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/appeal"
	"dommax/internal/app/apptest"
	"dommax/internal/app/issues"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

var msk = time.FixedZone("MSK", 3*60*60)

type fixture struct {
	store    *apptest.MemStore
	issues   *issues.Service
	svc      *appeal.Service
	now      *time.Time
	anna     user.User
	stranger user.User
	oper     user.User
	is       *issue.Issue
}

// setup: заявка о лифте подана в четверг 17.09, срок ответа до пятницы 18.09 включительно.
func setup(t *testing.T) fixture {
	t.Helper()
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК «Ореховый квартал»"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	s.Objects = []house.AssetObject{{ID: "h-1-e2-lift", HouseID: "h-1", Category: "lift", Label: "подъезд 2, пассажирский лифт", QRCode: "h-1-e2-lift"}}
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, msk)
	f := fixture{
		store:    s,
		now:      &now,
		anna:     user.User{ID: 1, Role: user.RoleResident, ConsentVersion: "v1"},
		stranger: user.User{ID: 2, Role: user.RoleResident, ConsentVersion: "v1"},
		oper:     user.User{ID: 3, Role: user.RoleOperator, OrganizationID: "org-1"},
	}
	for _, u := range []user.User{f.anna, f.stranger, f.oper} {
		s.AddUser(u)
	}
	clock := func() time.Time { return *f.now }
	f.issues = issues.NewService(s, issues.Config{Now: clock, NewID: func() string { return "i-1" }, ConsentVersion: "v1"})
	f.svc = appeal.NewService(s, appeal.Config{Secret: []byte("0123456789abcdef"), TTL: 10 * time.Minute, ConsentVersion: "v1", Now: clock})
	is, err := f.issues.Report(t.Context(), f.anna, issues.ReportInput{HouseID: "h-1", ObjectID: "h-1-e2-lift", Description: "Кабина не приходит"})
	if err != nil {
		t.Fatal(err)
	}
	f.is = is
	return f
}

func (f fixture) after(d time.Duration) { *f.now = f.now.Add(d) }

func TestParticipantGetsLinkAndDocument(t *testing.T) {
	f := setup(t)
	if _, err := f.issues.ChangeStatus(t.Context(), f.oper, f.is.ID(), issue.StatusAccepted, "Мастер приедет"); err != nil {
		t.Fatal(err)
	}
	f.after(3 * 24 * time.Hour) // воскресенье 20.09: срок истёк

	link, err := f.svc.Prepare(t.Context(), f.anna, f.is.ID())
	if err != nil || link.Token == "" || !link.ExpiresAt.Equal(f.now.Add(10*time.Minute)) {
		t.Fatalf("link = %+v, err = %v", link, err)
	}
	doc, err := f.svc.Document(t.Context(), link.Token)
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	if doc.Number != f.is.Number() || doc.Address != "Ореховый бульвар, 17к2" || doc.Place != "подъезд 2, пассажирский лифт" ||
		doc.Category != "Лифт" || doc.Organization != "УК «Ореховый квартал»" || doc.Participants != 1 ||
		!strings.Contains(doc.Basis, "416") || doc.Description != "Кабина не приходит" || !doc.Deadline.Equal(f.is.Deadline()) {
		t.Fatalf("document = %+v", doc)
	}
	if len(doc.Events) != 2 || doc.Events[1].Kind != issue.EventStatusChanged || doc.Events[1].Comment != "Мастер приедет" {
		t.Fatalf("events = %+v", doc.Events)
	}
}

// Коллективное обращение (ADR-023): соседи подписывают по просроченной заявке, ФИО по желанию,
// в документ идут число подписавших и таблица тех, кто указал ФИО.
func TestSignAppeal(t *testing.T) {
	f := setup(t)
	sergey := user.User{ID: 4, Role: user.RoleResident, HouseID: "h-1", ConsentVersion: "v1"}
	f.store.AddUser(sergey)
	if _, err := f.issues.Join(t.Context(), sergey, f.is.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Sign(t.Context(), f.anna, f.is.ID(), "", ""); !errors.Is(err, appeal.ErrNotOverdue) {
		t.Fatalf("before deadline err = %v", err)
	}
	f.after(3 * 24 * time.Hour)
	if _, err := f.svc.Sign(t.Context(), f.stranger, f.is.ID(), "", ""); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("stranger err = %v", err)
	}
	if _, err := f.svc.Sign(t.Context(), f.anna, f.is.ID(), strings.Repeat("я", 101), ""); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("long name err = %v", err)
	}
	noConsent := user.User{ID: 5, Role: user.RoleResident}
	if _, err := f.svc.Sign(t.Context(), noConsent, f.is.ID(), "", ""); !errors.Is(err, app.ErrConsentRequired) {
		t.Fatalf("no consent err = %v", err)
	}

	sum, err := f.svc.Sign(t.Context(), f.anna, f.is.ID(), " Анна Петрова ", "12")
	if err != nil || sum.Count != 1 || !sum.Mine || !sum.Named {
		t.Fatalf("anna signs = %+v, %v", sum, err)
	}
	// Квартира без ФИО не сохраняется: в таблице подписавших она ничего не значит.
	if sum, err = f.svc.Sign(t.Context(), sergey, f.is.ID(), "", "7"); err != nil || sum.Count != 2 || sum.Named {
		t.Fatalf("sergey signs = %+v, %v", sum, err)
	}
	link, err := f.svc.Prepare(t.Context(), f.anna, f.is.ID())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := f.svc.Document(t.Context(), link.Token)
	if err != nil || doc.Signed != 2 || len(doc.Signers) != 1 || doc.Signers[0] != (appeal.Signer{FullName: "Анна Петрова", Apartment: "12"}) {
		t.Fatalf("document signers = %d %+v, %v", doc.Signed, doc.Signers, err)
	}

	if sum, err = f.svc.Withdraw(t.Context(), sergey, f.is.ID()); err != nil || sum.Count != 1 || sum.Mine {
		t.Fatalf("withdraw = %+v, %v", sum, err)
	}
	// Удаление аккаунта стирает подписи.
	if err := f.store.Appeals().ForgetUser(t.Context(), f.anna.ID); err != nil {
		t.Fatal(err)
	}
	if sum, _ = f.svc.Summary(t.Context(), f.anna, f.is.ID()); sum.Count != 0 {
		t.Fatalf("after forget = %+v", sum)
	}
}

func TestPrepareRules(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Prepare(t.Context(), f.anna, f.is.ID()); !errors.Is(err, appeal.ErrNotOverdue) {
		t.Fatalf("before deadline err = %v, want ErrNotOverdue", err)
	}
	f.after(3 * 24 * time.Hour)
	if _, err := f.svc.Prepare(t.Context(), f.stranger, f.is.ID()); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("not a participant err = %v, want forbidden", err)
	}
	if _, err := f.svc.Prepare(t.Context(), f.anna, "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown issue err = %v, want not found", err)
	}
	for _, st := range []issue.Status{issue.StatusAccepted, issue.StatusDone} {
		if _, err := f.issues.ChangeStatus(t.Context(), f.oper, f.is.ID(), st, "Готово"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.Prepare(t.Context(), f.anna, f.is.ID()); !errors.Is(err, issue.ErrClosed) {
		t.Fatalf("closed issue err = %v, want ErrClosed", err)
	}
}

func TestLinkIsSignedAndShortLived(t *testing.T) {
	f := setup(t)
	f.after(3 * 24 * time.Hour)
	link, err := f.svc.Prepare(t.Context(), f.anna, f.is.ID())
	if err != nil {
		t.Fatal(err)
	}
	other := appeal.NewService(f.store, appeal.Config{Secret: []byte("another-secret-00"), TTL: time.Minute, Now: func() time.Time { return *f.now }})
	for name, tc := range map[string]struct {
		svc   *appeal.Service
		token string
	}{
		"чужая подпись":      {other, link.Token},
		"подделанные данные": {f.svc, "x" + link.Token},
		"мусор":              {f.svc, "abc"},
		"пусто":              {f.svc, ""},
	} {
		if _, err := tc.svc.Document(t.Context(), tc.token); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("%s: err = %v, want forbidden", name, err)
		}
	}
	f.after(11 * time.Minute)
	if _, err := f.svc.Document(t.Context(), link.Token); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("expired link err = %v, want forbidden", err)
	}
}

// Ссылка проверяет заявку заново: закрытую за эти 10 минут не выдаёт как просроченную.
func TestLinkRechecksIssueState(t *testing.T) {
	f := setup(t)
	f.after(3 * 24 * time.Hour)
	link, err := f.svc.Prepare(t.Context(), f.anna, f.is.ID())
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range []issue.Status{issue.StatusAccepted, issue.StatusDone} {
		if _, err := f.issues.ChangeStatus(t.Context(), f.oper, f.is.ID(), st, "Готово"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.Document(t.Context(), link.Token); !errors.Is(err, issue.ErrClosed) {
		t.Fatalf("closed after link err = %v, want ErrClosed", err)
	}
}

// Аккаунт удалён после выдачи ссылки: документ по ней больше не выдаётся.
func TestLinkOfDeletedAccountStopsWorking(t *testing.T) {
	f := setup(t)
	f.after(3 * 24 * time.Hour)
	link, err := f.svc.Prepare(t.Context(), f.anna, f.is.ID())
	if err != nil {
		t.Fatal(err)
	}
	gone := f.anna
	gone.Delete(*f.now)
	f.store.AddUser(gone)
	if _, err := f.svc.Document(t.Context(), link.Token); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("deleted account err = %v, want forbidden", err)
	}
}
