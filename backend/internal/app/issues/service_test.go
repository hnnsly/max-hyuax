package issues_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/issues"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

var msk = time.FixedZone("MSK", 3*60*60)

// Четверг 17.09.2026, 10:00 по Москве.
var now = time.Date(2026, 9, 17, 10, 0, 0, 0, msk)

type fixture struct {
	store              *apptest.MemStore
	svc                *issues.Service
	anna, sergey, oper user.User
}

func setup(t *testing.T) fixture {
	t.Helper()
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Type: house.OrgManagementCompany, Name: "УК"}
	s.Orgs["org-2"] = house.Organization{ID: "org-2", Type: house.OrgManagementCompany, Name: "Чужая УК"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	s.Objects = []house.AssetObject{{ID: "h-1-e2-lift", HouseID: "h-1", Category: "lift", Label: "подъезд 2, пассажирский лифт", QRCode: "h-1-e2-lift"}}

	f := fixture{
		store:  s,
		anna:   user.User{ID: 1, Role: user.RoleResident, ConsentVersion: "v1"},
		sergey: user.User{ID: 2, Role: user.RoleResident, ConsentVersion: "v1"},
		oper:   user.User{ID: 3, Role: user.RoleOperator, OrganizationID: "org-1"},
	}
	for _, u := range []user.User{f.anna, f.sergey, f.oper} {
		s.AddUser(u)
	}
	n := 0
	f.svc = issues.NewService(s, issues.Config{
		Now:            func() time.Time { return now },
		NewID:          func() string { n++; return fmt.Sprintf("id-%d", n) },
		ConsentVersion: "v1",
	})
	return f
}

func report(t *testing.T, f fixture, by user.User) *issue.Issue {
	t.Helper()
	is, err := f.svc.Report(t.Context(), by, issues.ReportInput{HouseID: "h-1", ObjectID: "h-1-e2-lift", Description: "Не приходит кабина"})
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	return is
}

func TestReportTakesCategoryFromObjectAndDeadlineFromRules(t *testing.T) {
	f := setup(t)
	is := report(t, f, f.anna)

	if is.Category() != "lift" || is.ResponsibleOrgID() != "org-1" || is.Number() == 0 {
		t.Fatalf("issue = category %q, org %q, number %d", is.Category(), is.ResponsibleOrgID(), is.Number())
	}
	// Место объекта показывается отдельной строкой, заголовок его не повторяет.
	if is.Title() != "Лифт" {
		t.Fatalf("title = %q, want category title", is.Title())
	}
	// Лифт — 1 рабочий день: из четверга до конца пятницы.
	if want := time.Date(2026, 9, 18, 23, 59, 59, 0, msk); !is.Deadline().Equal(want) {
		t.Fatalf("deadline = %v, want %v", is.Deadline(), want)
	}
	if len(f.store.Events) != 1 || f.store.Events[0].Kind != issue.EventCreated {
		t.Fatalf("events = %+v", f.store.Events)
	}
}

func TestReportRequiresConsent(t *testing.T) {
	f := setup(t)
	noConsent := user.User{ID: 9, Role: user.RoleResident}
	_, err := f.svc.Report(t.Context(), noConsent, issues.ReportInput{HouseID: "h-1", Category: "lift", Description: "x"})
	if !errors.Is(err, app.ErrConsentRequired) {
		t.Fatalf("err = %v, want ErrConsentRequired", err)
	}
}

func TestReportRejectsUnknownCategoryAndForeignObject(t *testing.T) {
	f := setup(t)
	_, err := f.svc.Report(t.Context(), f.anna, issues.ReportInput{HouseID: "h-1", Category: "teleport"})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("unknown category err = %v", err)
	}
	f.store.HouseMap["h-2"] = house.House{ID: "h-2", OrganizationID: "org-1"}
	_, err = f.svc.Report(t.Context(), f.anna, issues.ReportInput{HouseID: "h-2", ObjectID: "h-1-e2-lift"})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("object of another house err = %v", err)
	}
}

func TestSimilarFindsOpenIssueOfSameObject(t *testing.T) {
	f := setup(t)
	is := report(t, f, f.anna)

	got, err := f.svc.FindSimilar(t.Context(), "h-1", "lift", "h-1-e2-lift")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID() != is.ID() {
		t.Fatalf("similar = %v", got)
	}
}

func TestJoinAddsNeighbourOnce(t *testing.T) {
	f := setup(t)
	is := report(t, f, f.anna)

	joined, err := f.svc.Join(t.Context(), f.sergey, is.ID())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if joined.ParticipantCount() != 2 {
		t.Fatalf("participants = %d", joined.ParticipantCount())
	}
	if _, err := f.svc.Join(t.Context(), f.sergey, is.ID()); !errors.Is(err, issue.ErrAlreadyJoined) {
		t.Fatalf("second join err = %v", err)
	}
}

func TestOnlyOwnOperatorChangesStatus(t *testing.T) {
	f := setup(t)
	is := report(t, f, f.anna)

	if _, err := f.svc.ChangeStatus(t.Context(), f.anna, is.ID(), issue.StatusAccepted, ""); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("resident err = %v, want ErrForbidden", err)
	}
	foreign := user.User{ID: 4, Role: user.RoleOperator, OrganizationID: "org-2"}
	if _, err := f.svc.ChangeStatus(t.Context(), foreign, is.ID(), issue.StatusAccepted, ""); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("foreign operator err = %v, want ErrForbidden", err)
	}
	changed, err := f.svc.ChangeStatus(t.Context(), f.oper, is.ID(), issue.StatusInProgress, "Мастер едет")
	if err != nil {
		t.Fatalf("ChangeStatus: %v", err)
	}
	if changed.Status() != issue.StatusInProgress || changed.StatusComment() != "Мастер едет" {
		t.Fatalf("issue = %q %q", changed.Status(), changed.StatusComment())
	}
}

func TestQueueIsForOperatorsOnly(t *testing.T) {
	f := setup(t)
	report(t, f, f.anna)

	if _, err := f.svc.Queue(t.Context(), f.anna); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("resident queue err = %v", err)
	}
	q, err := f.svc.Queue(t.Context(), f.oper)
	if err != nil || len(q) != 1 {
		t.Fatalf("queue = %v, err = %v", q, err)
	}
}

func TestMetricsCountOwnOrganization(t *testing.T) {
	f := setup(t)
	a := report(t, f, f.anna)
	if _, err := f.svc.Join(t.Context(), f.sergey, a.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeStatus(t.Context(), f.oper, a.ID(), issue.StatusAccepted, ""); err != nil {
		t.Fatal(err)
	}
	b, c := report(t, f, f.sergey), report(t, f, f.anna)
	for _, id := range []string{b.ID(), c.ID()} {
		for _, st := range []issue.Status{issue.StatusAccepted, issue.StatusDone} {
			if _, err := f.svc.ChangeStatus(t.Context(), f.oper, id, st, "Готово"); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Ремонт по b жители подтвердили, c вернули в работу.
	if _, err := f.svc.Confirm(t.Context(), f.sergey, b.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Reopen(t.Context(), f.anna, c.ID(), "Не починили"); err != nil {
		t.Fatal(err)
	}

	m, err := f.svc.Metrics(t.Context(), f.oper)
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	want := app.OrgCounts{Issues: 3, Reports: 4, ClosedTotal: 1, ClosedOnTime: 1, Confirmed: 1, Reopened: 1, OpenTotal: 2}
	if m.OrgCounts != want {
		t.Errorf("counts = %+v, want %+v", m.OrgCounts, want)
	}
	if m.Week == nil || *m.Week != 0 || m.PrevWeek != nil || len(m.ByDay) != 7 || m.ByDay[6].Median == nil {
		t.Errorf("first response: week=%v prev=%v days=%+v", m.Week, m.PrevWeek, m.ByDay)
	}

	other := user.User{ID: 9, Role: user.RoleOperator, OrganizationID: "org-2"}
	if m, err := f.svc.Metrics(t.Context(), other); err != nil || m.Issues != 0 || m.Week != nil {
		t.Errorf("other org metrics = %+v, err = %v", m, err)
	}
	for _, u := range []user.User{f.anna, {ID: 10, Role: user.RoleOperator}} {
		if _, err := f.svc.Metrics(t.Context(), u); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("Metrics(%+v) err = %v, want forbidden", u, err)
		}
	}
}

func TestMineAndTimeline(t *testing.T) {
	f := setup(t)
	is := report(t, f, f.anna)
	if _, err := f.svc.Join(t.Context(), f.sergey, is.ID()); err != nil {
		t.Fatal(err)
	}
	mine, err := f.svc.Mine(t.Context(), f.sergey)
	if err != nil || len(mine) != 1 {
		t.Fatalf("mine = %v, err = %v", mine, err)
	}
	tl, err := f.svc.Timeline(t.Context(), is.ID())
	if err != nil || len(tl) != 2 || tl[1].Kind != issue.EventJoined {
		t.Fatalf("timeline = %+v, err = %v", tl, err)
	}
	if _, err := f.svc.Timeline(t.Context(), "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown issue timeline err = %v", err)
	}
}

func TestChangesEnqueueLiveCardNotifications(t *testing.T) {
	f := setup(t)
	pending := func() []app.Notification { return f.store.Outbox().(*apptest.MemOutbox).Pending }
	is := report(t, f, f.anna)
	if want := []app.Notification{{Kind: app.NotifyCard, IssueID: is.ID(), UserID: f.anna.ID}}; !slices.Equal(pending(), want) {
		t.Fatalf("after report = %+v", pending())
	}
	if _, err := f.svc.Join(t.Context(), f.sergey, is.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeStatus(t.Context(), f.oper, is.ID(), issue.StatusInProgress, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeStatus(t.Context(), f.oper, is.ID(), issue.StatusDone, "Починили"); err != nil {
		t.Fatal(err)
	}
	var finals int
	for _, n := range pending() {
		if n.Kind == app.NotifyFinal {
			finals++
		}
	}
	// Карточки Анны и Сергея схлопываются в очереди, итоговых сообщений два.
	if len(pending()) != 4 || finals != 2 {
		t.Fatalf("pending = %+v", pending())
	}
}

func TestMarkOverdueNotifiesOnce(t *testing.T) {
	f := setup(t)
	is := report(t, f, f.anna)
	f.store.Outbox().(*apptest.MemOutbox).Pending = nil

	// Лифт — 1 рабочий день: через трое суток срок точно прошёл.
	later := issues.NewService(f.store, issues.Config{Now: func() time.Time { return now.Add(72 * time.Hour) }, NewID: func() string { return "x" }, ConsentVersion: "v1"})
	n, err := later.MarkOverdue(t.Context())
	if err != nil || n != 1 {
		t.Fatalf("MarkOverdue = %d, %v", n, err)
	}
	got, _ := f.svc.Get(t.Context(), is.ID())
	if got.OverdueAt().IsZero() {
		t.Fatal("overdue mark must be saved")
	}
	pending := f.store.Outbox().(*apptest.MemOutbox).Pending
	if !slices.Contains(pending, app.Notification{Kind: app.NotifyOverdue, IssueID: is.ID(), UserID: f.anna.ID}) {
		t.Fatalf("pending = %+v", pending)
	}
	if n, err := later.MarkOverdue(t.Context()); err != nil || n != 0 {
		t.Fatalf("second run = %d, %v; want nothing to do", n, err)
	}
}

// brokenIssue — хранилище, где одна заявка не читается под блокировкой (сбой базы на ней).
type brokenIssue struct {
	*apptest.MemStore
	id string
}

func (s brokenIssue) Issues() app.IssueRepo { return brokenRepo{s.MemStore.Issues(), s.id} }
func (s brokenIssue) InTx(ctx context.Context, fn func(app.Store) error) error {
	return fn(s)
}

type brokenRepo struct {
	app.IssueRepo
	id string
}

func (r brokenRepo) GetForUpdate(ctx context.Context, id string) (*issue.Issue, error) {
	if id == r.id {
		return nil, errors.New("row is broken")
	}
	return r.IssueRepo.GetForUpdate(ctx, id)
}

// Сбой на одной заявке не мешает отметить остальные: ошибка возвращается, но пакет доходит до конца.
func TestMarkOverdueSkipsBrokenIssue(t *testing.T) {
	f := setup(t)
	report(t, f, f.anna)
	report(t, f, f.sergey)
	at := now.Add(72 * time.Hour)
	// Ломаем ту заявку, которую задача возьмёт первой.
	list, _ := f.store.Issues().ListOverdueUnmarked(t.Context(), at, 10)
	store := brokenIssue{f.store, list[0].ID()}
	later := issues.NewService(store, issues.Config{Now: func() time.Time { return at }, NewID: func() string { return "x" }, ConsentVersion: "v1"})
	n, err := later.MarkOverdue(t.Context())
	if err == nil || n != 1 {
		t.Fatalf("MarkOverdue = %d, %v; want 1 marked and the error", n, err)
	}
	if got, _ := f.svc.Get(t.Context(), list[1].ID()); got.OverdueAt().IsZero() {
		t.Fatal("the healthy issue must be marked")
	}
}

func TestGetUnknownIssue(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Get(t.Context(), "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
