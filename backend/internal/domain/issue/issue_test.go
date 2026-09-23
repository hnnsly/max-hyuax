package issue_test

import (
	"errors"
	"testing"
	"time"

	"dommax/internal/domain/issue"
)

var (
	created  = time.Date(2026, 9, 17, 8, 10, 0, 0, time.UTC)
	deadline = time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
)

func newIssue(t *testing.T) *issue.Issue {
	t.Helper()
	is, err := issue.New(issue.NewParams{
		ID:          "0142",
		HouseID:     "house-17k2",
		ObjectID:    "lift-e2",
		Category:    "lift",
		Title:       "Лифт не работает",
		Description: "Кабина не приходит",
		ReporterID:  1001,
		CreatedAt:   created,
		Deadline:    deadline,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return is
}

func TestNewIssueStartsAsSentWithReporterAsParticipant(t *testing.T) {
	is := newIssue(t)

	if is.Status() != issue.StatusSent {
		t.Fatalf("status = %q, want %q", is.Status(), issue.StatusSent)
	}
	if is.ParticipantCount() != 1 || !is.HasParticipant(1001) {
		t.Fatalf("participants = %d, reporter present = %v", is.ParticipantCount(), is.HasParticipant(1001))
	}
	events := is.PullEvents()
	if len(events) != 1 || events[0].Kind != issue.EventCreated {
		t.Fatalf("events = %+v, want one Created", events)
	}
}

func TestNewIssueRequiresTitleAndCategory(t *testing.T) {
	_, err := issue.New(issue.NewParams{ID: "1", HouseID: "h", ReporterID: 1, CreatedAt: created, Deadline: deadline})
	if !errors.Is(err, issue.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestJoinAddsParticipantOnce(t *testing.T) {
	is := newIssue(t)
	is.PullEvents()

	if err := is.Join(2002, created.Add(time.Hour)); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if is.ParticipantCount() != 2 {
		t.Fatalf("participants = %d, want 2", is.ParticipantCount())
	}
	if err := is.Join(2002, created.Add(2*time.Hour)); !errors.Is(err, issue.ErrAlreadyJoined) {
		t.Fatalf("second Join err = %v, want ErrAlreadyJoined", err)
	}
	events := is.PullEvents()
	if len(events) != 1 || events[0].Kind != issue.EventJoined || events[0].UserID != 2002 {
		t.Fatalf("events = %+v, want one Joined by 2002", events)
	}
}

func TestJoinClosedIssueIsRejected(t *testing.T) {
	is := newIssue(t)
	if err := is.ChangeStatus(issue.StatusRejected, "Не наш участок, звоните в Мосводоканал", created); err != nil {
		t.Fatalf("ChangeStatus: %v", err)
	}
	if err := is.Join(2002, created); !errors.Is(err, issue.ErrClosed) {
		t.Fatalf("err = %v, want ErrClosed", err)
	}
}

func TestStatusTransitions(t *testing.T) {
	cases := []struct {
		name    string
		path    []issue.Status
		wantErr error
	}{
		{"accept then work then done", []issue.Status{issue.StatusAccepted, issue.StatusInProgress, issue.StatusDone}, nil},
		{"sent straight to in progress", []issue.Status{issue.StatusInProgress}, nil},
		{"sent straight to done is not allowed", []issue.Status{issue.StatusDone}, issue.ErrTransition},
		{"done is terminal", []issue.Status{issue.StatusAccepted, issue.StatusDone, issue.StatusInProgress}, issue.ErrTransition},
		{"same status is not a change", []issue.Status{issue.StatusAccepted, issue.StatusAccepted}, issue.ErrTransition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			is := newIssue(t)
			var err error
			for _, st := range tc.path {
				if err = is.ChangeStatus(st, "", created); err != nil {
					break
				}
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRejectRequiresReason(t *testing.T) {
	is := newIssue(t)
	if err := is.ChangeStatus(issue.StatusRejected, "  ", created); !errors.Is(err, issue.ErrReasonRequired) {
		t.Fatalf("err = %v, want ErrReasonRequired", err)
	}
}

func TestChangeStatusRecordsEventWithComment(t *testing.T) {
	is := newIssue(t)
	is.PullEvents()
	at := created.Add(3 * time.Hour)

	if err := is.ChangeStatus(issue.StatusAccepted, "Мастер приедет завтра", at); err != nil {
		t.Fatalf("ChangeStatus: %v", err)
	}
	events := is.PullEvents()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Kind != issue.EventStatusChanged || ev.Status != issue.StatusAccepted || ev.Comment != "Мастер приедет завтра" || !ev.At.Equal(at) {
		t.Fatalf("event = %+v", ev)
	}
}

func TestPendingEventsDoesNotClear(t *testing.T) {
	is := newIssue(t)
	if got := is.PendingEvents(); len(got) != 1 || got[0].Kind != issue.EventCreated {
		t.Fatalf("pending = %+v", got)
	}
	if len(is.PullEvents()) != 1 {
		t.Fatal("PendingEvents must not clear events")
	}
}

func TestRestoreKeepsStateWithoutEvents(t *testing.T) {
	at := created.Add(time.Hour)
	is := issue.Restore(issue.NewParams{ID: "u-1", HouseID: "h", Category: "lift", Title: "Лифт", ResponsibleOrgID: "org-1", CreatedAt: created, Deadline: deadline},
		issue.State{Number: 142, Status: issue.StatusAccepted, StatusAt: at, StatusComment: "Мастер завтра", Participants: []issue.Participant{{UserID: 1}, {UserID: 2}}})

	if is.Status() != issue.StatusAccepted || is.ParticipantCount() != 2 || is.Number() != 142 {
		t.Fatalf("status = %q, participants = %d, number = %d", is.Status(), is.ParticipantCount(), is.Number())
	}
	if !is.StatusAt().Equal(at) || is.StatusComment() != "Мастер завтра" || is.ResponsibleOrgID() != "org-1" {
		t.Fatalf("status at = %v, comment = %q, org = %q", is.StatusAt(), is.StatusComment(), is.ResponsibleOrgID())
	}
	if len(is.PullEvents()) != 0 {
		t.Fatal("restored issue must not carry events")
	}
}

func TestChangeStatusUpdatesStatusTimeAndComment(t *testing.T) {
	is := newIssue(t)
	if !is.StatusAt().Equal(created) {
		t.Fatalf("new issue status at = %v, want creation time", is.StatusAt())
	}
	at := created.Add(2 * time.Hour)
	if err := is.ChangeStatus(issue.StatusInProgress, "Мастер на месте", at); err != nil {
		t.Fatal(err)
	}
	if !is.StatusAt().Equal(at) || is.StatusComment() != "Мастер на месте" {
		t.Fatalf("status at = %v, comment = %q", is.StatusAt(), is.StatusComment())
	}
}

func TestOverdue(t *testing.T) {
	is := newIssue(t)
	if is.IsOverdue(deadline.Add(-time.Minute)) {
		t.Fatal("overdue before deadline")
	}
	if !is.IsOverdue(deadline.Add(time.Minute)) {
		t.Fatal("not overdue after deadline")
	}
	if err := is.ChangeStatus(issue.StatusInProgress, "", created); err != nil {
		t.Fatal(err)
	}
	if err := is.ChangeStatus(issue.StatusDone, "", created); err != nil {
		t.Fatal(err)
	}
	if is.IsOverdue(deadline.Add(48 * time.Hour)) {
		t.Fatal("closed issue must not be overdue")
	}
}
