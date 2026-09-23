//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
	"uuid"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
	"dommax/internal/storage/postgres"
	"dommax/internal/storage/postgres/pgtest"
)

var store *postgres.Store

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn, drop, err := pgtest.Fresh(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration db:", err)
		os.Exit(1)
	}
	store, err = postgres.Open(ctx, dsn)
	if err != nil {
		drop()
		fmt.Fprintln(os.Stderr, "open store:", err)
		os.Exit(1)
	}
	code := m.Run()
	store.Close()
	drop()
	os.Exit(code)
}

func demoUser(t *testing.T, key string) user.User {
	t.Helper()
	u, err := store.Users().ByDemoKey(t.Context(), key)
	if err != nil {
		t.Fatalf("demo user %s: %v", key, err)
	}
	return u
}

func newIssue(t *testing.T, reporter int64) *issue.Issue {
	t.Helper()
	now := time.Now().Truncate(time.Microsecond)
	is, err := issue.New(issue.NewParams{
		ID: uuid.NewV7().String(), HouseID: "h-17k2", ObjectID: "h-17k2-e2-lift", Category: "lift",
		Title: "Лифт", Description: "Не приходит", ResponsibleOrgID: "org-orekh",
		ReporterID: reporter, CreatedAt: now, Deadline: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	return is
}

func TestSeedHasDemoData(t *testing.T) {
	houses, err := store.Houses().Search(t.Context(), "17к2")
	if err != nil || len(houses) != 1 || houses[0].OrganizationID != "org-orekh" {
		t.Fatalf("houses = %+v, err = %v", houses, err)
	}
	obj, err := store.Houses().ObjectByCode(t.Context(), "h-17k2-e2-lift")
	if err != nil || obj.HouseID != "h-17k2" || obj.EntranceID != "h-17k2-e2" || obj.Category != "lift" {
		t.Fatalf("object = %+v, err = %v", obj, err)
	}
	near, err := store.Houses().Nearest(t.Context(), 55.6124, 37.7462, 1)
	if err != nil || len(near) != 1 || near[0].ID != "h-17k2" {
		t.Fatalf("nearest = %+v, err = %v", near, err)
	}
	op := demoUser(t, "uk_operator_demo")
	if op.Role != user.RoleOperator || op.OrganizationID != "org-orekh" {
		t.Fatalf("operator = %+v", op)
	}
}

func TestIssueRoundTripWithParticipantsAndNumber(t *testing.T) {
	anna, sergey := demoUser(t, "resident_demo_1"), demoUser(t, "resident_demo_2")
	is := newIssue(t, anna.ID)
	if err := store.Issues().Create(t.Context(), is); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if is.Number() < 101 {
		t.Fatalf("number = %d, want from sequence", is.Number())
	}

	err := store.InTx(t.Context(), func(tx app.Store) error {
		got, err := tx.Issues().GetForUpdate(t.Context(), is.ID())
		if err != nil {
			return err
		}
		if err := got.Join(sergey.ID, time.Now()); err != nil {
			return err
		}
		if err := got.ChangeStatus(issue.StatusInProgress, "Мастер едет", time.Now()); err != nil {
			return err
		}
		return tx.Issues().Save(t.Context(), got)
	})
	if err != nil {
		t.Fatalf("update in tx: %v", err)
	}

	got, err := store.Issues().Get(t.Context(), is.ID())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Number() != is.Number() || got.ParticipantCount() != 2 || got.ReporterID() != anna.ID ||
		got.Status() != issue.StatusInProgress || got.StatusComment() != "Мастер едет" || got.ObjectID() != "h-17k2-e2-lift" {
		t.Fatalf("got = number %d, participants %d, reporter %d, status %q, comment %q, object %q",
			got.Number(), got.ParticipantCount(), got.ReporterID(), got.Status(), got.StatusComment(), got.ObjectID())
	}
	if len(got.PullEvents()) != 0 {
		t.Fatal("loaded issue must not carry events")
	}
}

func TestFailedTransactionRollsBack(t *testing.T) {
	anna := demoUser(t, "resident_demo_1")
	is := newIssue(t, anna.ID)
	if err := store.Issues().Create(t.Context(), is); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := store.InTx(t.Context(), func(tx app.Store) error {
		got, _ := tx.Issues().GetForUpdate(t.Context(), is.ID())
		_ = got.ChangeStatus(issue.StatusAccepted, "", time.Now())
		if err := tx.Issues().Save(t.Context(), got); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	got, _ := store.Issues().Get(t.Context(), is.ID())
	if got.Status() != issue.StatusSent {
		t.Fatalf("status = %q, want rollback to sent", got.Status())
	}
}

func TestListsAndSimilar(t *testing.T) {
	anna := demoUser(t, "resident_demo_1")
	is := newIssue(t, anna.ID)
	if err := store.Issues().Create(t.Context(), is); err != nil {
		t.Fatal(err)
	}
	similar, err := store.Issues().FindSimilar(t.Context(), "h-17k2", "lift", "h-17k2-e2-lift", time.Now().Add(-time.Hour))
	if err != nil || len(similar) == 0 || similar[0].ParticipantCount() == 0 {
		t.Fatalf("similar = %v, err = %v", similar, err)
	}
	house, err := store.Issues().ListByHouse(t.Context(), "h-17k2", 50)
	if err != nil || len(house) < 4 {
		t.Fatalf("house issues = %d, err = %v", len(house), err)
	}
	queue, err := store.Issues().Queue(t.Context(), "org-orekh", 50)
	if err != nil || len(queue) < 4 || queue[len(queue)-1].Status() != issue.StatusDone {
		t.Fatalf("queue = %d, err = %v (closed issues must go last)", len(queue), err)
	}
}

func TestParticipantIssuesAndEvents(t *testing.T) {
	anna, sergey := demoUser(t, "resident_demo_1"), demoUser(t, "resident_demo_2")
	is := newIssue(t, anna.ID)
	if err := store.Issues().Create(t.Context(), is); err != nil {
		t.Fatal(err)
	}
	_ = is.Join(sergey.ID, time.Now())
	if err := store.Issues().Save(t.Context(), is); err != nil {
		t.Fatal(err)
	}
	mine, err := store.Issues().ListByParticipant(t.Context(), sergey.ID, 50)
	if err != nil || len(mine) == 0 || !mine[0].HasParticipant(sergey.ID) {
		t.Fatalf("mine = %v, err = %v", mine, err)
	}
	events, err := store.Issues().Events(t.Context(), is.ID())
	if err != nil || len(events) != 2 || events[0].Kind != issue.EventCreated || events[1].Kind != issue.EventJoined || events[1].UserID != sergey.ID {
		t.Fatalf("events = %+v, err = %v", events, err)
	}
}

func TestUnknownIssueIsNotFound(t *testing.T) {
	if _, err := store.Issues().Get(t.Context(), uuid.NewV7().String()); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestUserCreateConsentAndDelete(t *testing.T) {
	u, err := store.Users().Create(t.Context(), user.User{MaxUserID: 555001, FirstName: "Ольга", Role: user.RoleResident})
	if err != nil || u.ID == 0 {
		t.Fatalf("Create: %+v, %v", u, err)
	}
	u.AcceptConsent("v1", time.Now())
	u.HouseID = "h-15"
	if err := store.Users().Save(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	got, err := store.Users().ByMaxID(t.Context(), 555001)
	if err != nil || !got.HasConsent("v1") || got.HouseID != "h-15" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
	got.Delete(time.Now())
	if err := store.Users().Save(t.Context(), got); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Users().Get(t.Context(), u.ID)
	if !got.Deleted() || got.FirstName != "" {
		t.Fatalf("deleted user = %+v", got)
	}
}

func TestMarkUpdateProcessedOnce(t *testing.T) {
	first, err := store.MarkUpdateProcessed(t.Context(), "message_created:1:m1")
	if err != nil || !first {
		t.Fatalf("first = %v, err = %v", first, err)
	}
	again, err := store.MarkUpdateProcessed(t.Context(), "message_created:1:m1")
	if err != nil || again {
		t.Fatalf("again = %v, err = %v", again, err)
	}
}
