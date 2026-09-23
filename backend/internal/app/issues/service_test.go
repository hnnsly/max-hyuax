package issues_test

import (
	"errors"
	"fmt"
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

func TestGetUnknownIssue(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.Get(t.Context(), "nope"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
