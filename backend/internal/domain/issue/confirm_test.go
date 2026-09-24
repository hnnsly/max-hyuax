package issue_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"dommax/internal/domain/issue"
)

// doneIssue — заявка Анны (1001), к которой присоединился Сергей (1002); УК отметила её выполненной.
func doneIssue(t *testing.T) (*issue.Issue, time.Time) {
	t.Helper()
	is := newIssue(t)
	if err := is.Join(1002, created.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	doneAt := created.Add(24 * time.Hour)
	for _, st := range []issue.Status{issue.StatusAccepted, issue.StatusDone} {
		if err := is.ChangeStatus(st, "Лифт починили", doneAt); err != nil {
			t.Fatal(err)
		}
	}
	is.PullEvents()
	return is, doneAt
}

func TestParticipantConfirmsRepairOnce(t *testing.T) {
	is, doneAt := doneIssue(t)
	at := doneAt.Add(2 * time.Hour)
	if err := is.Confirm(1002, at); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if got := is.ConfirmedCount(); got != 1 {
		t.Fatalf("confirmed = %d, want 1", got)
	}
	if a, ok := is.AnswerOf(1002); !ok || !a.Fixed {
		t.Fatalf("answer of 1002 = %+v, %v", a, ok)
	}
	ev := is.PullEvents()
	if len(ev) != 1 || ev[0].Kind != issue.EventConfirmed || ev[0].UserID != 1002 || !ev[0].At.Equal(at) {
		t.Fatalf("events = %+v", ev)
	}
	// Один ответ от человека: второй раз ни подтвердить, ни вернуть нельзя.
	if err := is.Confirm(1002, at); !errors.Is(err, issue.ErrAlreadyAnswered) {
		t.Fatalf("second confirm err = %v, want ErrAlreadyAnswered", err)
	}
	if err := is.Reopen(1002, "Опять стоит", at, at.Add(48*time.Hour)); !errors.Is(err, issue.ErrAlreadyAnswered) {
		t.Fatalf("reopen after confirm err = %v, want ErrAlreadyAnswered", err)
	}
	if is.Status() != issue.StatusDone {
		t.Fatalf("status = %s, want done", is.Status())
	}
}

func TestConfirmRules(t *testing.T) {
	open := newIssue(t)
	if err := open.Confirm(1001, created.Add(time.Hour)); !errors.Is(err, issue.ErrNotDone) {
		t.Fatalf("open issue err = %v, want ErrNotDone", err)
	}
	// Чужому жителю отвечаем «не участник», даже если заявка ещё не выполнена: состояние чужой заявки ему не важно.
	if err := open.Confirm(4242, created.Add(time.Hour)); !errors.Is(err, issue.ErrNotParticipant) {
		t.Fatalf("stranger on open issue err = %v, want ErrNotParticipant", err)
	}
	is, doneAt := doneIssue(t)
	if err := is.Confirm(4242, doneAt.Add(time.Hour)); !errors.Is(err, issue.ErrNotParticipant) {
		t.Fatalf("stranger err = %v, want ErrNotParticipant", err)
	}
	if err := is.Confirm(1001, doneAt.Add(issue.ConfirmWindow+time.Minute)); !errors.Is(err, issue.ErrWindowClosed) {
		t.Fatalf("late err = %v, want ErrWindowClosed", err)
	}
	if err := is.Confirm(1001, doneAt.Add(issue.ConfirmWindow)); err != nil {
		t.Fatalf("last moment of the window: %v", err)
	}
}

// Отказ УК жители не подтверждают: подтверждается только выполненный ремонт.
func TestRejectedIssueCannotBeConfirmed(t *testing.T) {
	is := newIssue(t)
	if err := is.ChangeStatus(issue.StatusRejected, "Не наш участок", created.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := is.Confirm(1001, created.Add(2*time.Hour)); !errors.Is(err, issue.ErrNotDone) {
		t.Fatalf("err = %v, want ErrNotDone", err)
	}
}

func TestParticipantReopensWithComment(t *testing.T) {
	is, doneAt := doneIssue(t)
	at := doneAt.Add(3 * time.Hour)
	newDeadline := at.Add(48 * time.Hour)
	if err := is.Reopen(1001, "  ", at, newDeadline); !errors.Is(err, issue.ErrCommentRequired) {
		t.Fatalf("empty comment err = %v, want ErrCommentRequired", err)
	}
	// Комментарий уходит в каждую хронологию: длиннее предела не принимаем.
	if err := is.Reopen(1001, strings.Repeat("я", issue.MaxCommentRunes+1), at, newDeadline); !errors.Is(err, issue.ErrInvalid) {
		t.Fatalf("long comment err = %v, want ErrInvalid", err)
	}
	if err := is.Reopen(1001, " Лифт снова стоит на 5 этаже ", at, newDeadline); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if is.Status() != issue.StatusInProgress || !is.StatusAt().Equal(at) || is.StatusComment() != "" {
		t.Fatalf("status = %s at %v, comment %q", is.Status(), is.StatusAt(), is.StatusComment())
	}
	if !is.Deadline().Equal(newDeadline) || !is.ReopenedAt().Equal(at) || !is.OverdueAt().IsZero() {
		t.Fatalf("deadline = %v, reopened = %v, overdue = %v", is.Deadline(), is.ReopenedAt(), is.OverdueAt())
	}
	ev := is.PullEvents()
	if len(ev) != 1 || ev[0].Kind != issue.EventReopened || ev[0].UserID != 1001 || ev[0].Comment != "Лифт снова стоит на 5 этаже" || ev[0].Status != issue.StatusInProgress {
		t.Fatalf("events = %+v", ev)
	}
	// Ответ «не починили» сохраняется для метрик, но относится к прошлому «выполнено».
	answers := is.NewAnswers()
	if len(answers) != 1 || answers[0].Fixed || !answers[0].DoneAt.Equal(doneAt) {
		t.Fatalf("new answers = %+v", answers)
	}
	// Заявка снова в работе: подтверждать нечего, пока УК не отметит её выполненной.
	if err := is.Confirm(1002, at.Add(time.Minute)); !errors.Is(err, issue.ErrNotDone) {
		t.Fatalf("confirm after reopen err = %v, want ErrNotDone", err)
	}
}

// После повторного «выполнено» жители отвечают заново: прошлые ответы относятся к прошлому кругу.
func TestAnswersStartOverAfterNextDone(t *testing.T) {
	is, doneAt := doneIssue(t)
	if err := is.Confirm(1002, doneAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	at := doneAt.Add(2 * time.Hour)
	if err := is.Reopen(1001, "Не работает", at, at.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	again := at.Add(24 * time.Hour)
	if err := is.ChangeStatus(issue.StatusDone, "Заменили плату", again); err != nil {
		t.Fatalf("done again: %v", err)
	}
	if is.ConfirmedCount() != 0 {
		t.Fatalf("confirmed = %d, want 0 in the new round", is.ConfirmedCount())
	}
	if err := is.Confirm(1002, again.Add(time.Hour)); err != nil {
		t.Fatalf("confirm in the new round: %v", err)
	}
}

func TestRestoreKeepsAnswersOfCurrentRound(t *testing.T) {
	doneAt := created.Add(24 * time.Hour)
	is := issue.Restore(issue.NewParams{ID: "x", Deadline: deadline}, issue.State{
		Status: issue.StatusDone, StatusAt: doneAt,
		Participants: []issue.Participant{{UserID: 1001}, {UserID: 1002}},
		Answers:      []issue.Answer{{UserID: 1001, Fixed: true, DoneAt: doneAt, At: doneAt.Add(time.Hour)}},
		ReopenedAt:   created.Add(12 * time.Hour),
	})
	if is.ConfirmedCount() != 1 || !is.ReopenedAt().Equal(created.Add(12*time.Hour)) {
		t.Fatalf("confirmed = %d, reopened = %v", is.ConfirmedCount(), is.ReopenedAt())
	}
	if err := is.Confirm(1001, doneAt.Add(2*time.Hour)); !errors.Is(err, issue.ErrAlreadyAnswered) {
		t.Fatalf("err = %v, want ErrAlreadyAnswered", err)
	}
	if len(is.NewAnswers()) != 0 {
		t.Fatal("restored answers are not new")
	}
}
