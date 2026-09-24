package issues_test

import (
	"errors"
	"slices"
	"testing"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
)

// done — заявка Анны, к которой присоединился Сергей; УК отметила её выполненной. Очередь outbox пуста.
func done(t *testing.T, f fixture) *issue.Issue {
	t.Helper()
	is := report(t, f, f.anna)
	if _, err := f.svc.Join(t.Context(), f.sergey, is.ID()); err != nil {
		t.Fatal(err)
	}
	for _, st := range []issue.Status{issue.StatusInProgress, issue.StatusDone} {
		if _, err := f.svc.ChangeStatus(t.Context(), f.oper, is.ID(), st, "Починили"); err != nil {
			t.Fatal(err)
		}
	}
	f.store.Outbox().(*apptest.MemOutbox).Pending = nil
	return is
}

// Подтверждение ничего не рассылает: соседям не нужно сообщение о каждом «починили».
func TestConfirmRepair(t *testing.T) {
	f := setup(t)
	is := done(t, f)
	got, err := f.svc.Confirm(t.Context(), f.sergey, is.ID())
	if err != nil || got.ConfirmedCount() != 1 {
		t.Fatalf("confirm = %v, err = %v", got, err)
	}
	if p := f.store.Outbox().(*apptest.MemOutbox).Pending; len(p) != 0 {
		t.Fatalf("pending = %+v, want nothing", p)
	}
	if again, _ := f.svc.Get(t.Context(), is.ID()); again.ConfirmedCount() != 1 {
		t.Fatal("confirmation must be saved")
	}
	// Сотрудник УК не участник: подтверждать ремонт за жителей он не может.
	if _, err := f.svc.Confirm(t.Context(), f.oper, is.ID()); !errors.Is(err, issue.ErrNotParticipant) {
		t.Fatalf("operator err = %v, want ErrNotParticipant", err)
	}
}

// Возврат в работу: срок заново по справочнику, карточки у всех, отдельное сообщение соседям.
func TestReopenRepair(t *testing.T) {
	f := setup(t)
	is := done(t, f)
	if _, err := f.svc.Reopen(t.Context(), f.anna, is.ID(), ""); !errors.Is(err, issue.ErrCommentRequired) {
		t.Fatalf("empty comment err = %v", err)
	}
	got, err := f.svc.Reopen(t.Context(), f.anna, is.ID(), "Лифт снова стоит")
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	rule, _ := rules.Lookup("lift")
	if got.Status() != issue.StatusInProgress || !got.Deadline().Equal(rule.Deadline(now.In(msk))) || got.ReopenedAt().IsZero() {
		t.Fatalf("reopened = %s, deadline %v, reopened at %v", got.Status(), got.Deadline(), got.ReopenedAt())
	}
	pending := f.store.Outbox().(*apptest.MemOutbox).Pending
	want := []app.Notification{
		{Kind: app.NotifyCard, IssueID: is.ID(), UserID: f.anna.ID},
		{Kind: app.NotifyCard, IssueID: is.ID(), UserID: f.sergey.ID},
		{Kind: app.NotifyReopened, IssueID: is.ID(), UserID: f.sergey.ID},
	}
	if !slices.Equal(pending, want) {
		t.Fatalf("pending = %+v, want %+v", pending, want)
	}
	events, _ := f.svc.Timeline(t.Context(), is.ID())
	last := events[len(events)-1]
	if last.Kind != issue.EventReopened || last.Comment != "Лифт снова стоит" {
		t.Fatalf("last event = %+v", last)
	}
}
