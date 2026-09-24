package issue

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ConfirmWindow — сколько дней после отметки «выполнено» жители могут подтвердить ремонт или вернуть заявку.
const ConfirmWindow = 7 * 24 * time.Hour

var (
	ErrNotDone         = errors.New("issue: repair is not marked done")
	ErrNotParticipant  = errors.New("issue: only participants answer")
	ErrWindowClosed    = errors.New("issue: confirmation window is closed")
	ErrAlreadyAnswered = errors.New("issue: user already answered")
	ErrCommentRequired = errors.New("issue: comment required to reopen")
)

// Answer — ответ участника на «выполнено»: починили или нет. DoneAt — к какой отметке «выполнено»
// относится ответ: после возврата в работу и нового «выполнено» жители отвечают заново.
type Answer struct {
	UserID int64
	Fixed  bool
	DoneAt time.Time
	At     time.Time
}

// Confirm — участник подтверждает, что ремонт сделан.
func (is *Issue) Confirm(userID int64, at time.Time) error {
	if err := is.canAnswer(userID, at); err != nil {
		return err
	}
	is.answer(Answer{UserID: userID, Fixed: true, DoneAt: is.statusAt, At: at})
	is.record(Event{Kind: EventConfirmed, UserID: userID, Status: is.status, At: at})
	return nil
}

// Reopen — участник сообщает, что не починили: заявка возвращается в работу с новым сроком
// по справочнику. Комментарий обязателен: УК должна понять, что осталось не так.
func (is *Issue) Reopen(userID int64, comment string, at, newDeadline time.Time) error {
	if err := is.canAnswer(userID, at); err != nil {
		return err
	}
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return ErrCommentRequired
	}
	if !newDeadline.After(at) {
		return fmt.Errorf("%w: new deadline must be after reopening", ErrInvalid)
	}
	is.answer(Answer{UserID: userID, Fixed: false, DoneAt: is.statusAt, At: at})
	is.status, is.statusAt, is.statusComment = StatusInProgress, at, ""
	is.deadline, is.overdueAt, is.reopenedAt = newDeadline, time.Time{}, at
	is.answers = nil // круг ответов закончен: следующий начнётся с нового «выполнено»
	is.record(Event{Kind: EventReopened, UserID: userID, Status: is.status, Comment: comment, At: at})
	return nil
}

func (is *Issue) canAnswer(userID int64, at time.Time) error {
	switch {
	case is.status != StatusDone:
		return ErrNotDone
	case !is.HasParticipant(userID):
		return ErrNotParticipant
	case at.After(is.statusAt.Add(ConfirmWindow)):
		return ErrWindowClosed
	}
	if _, ok := is.AnswerOf(userID); ok {
		return ErrAlreadyAnswered
	}
	return nil
}

func (is *Issue) answer(a Answer) {
	is.answers = append(is.answers, a)
	is.newAnswers = append(is.newAnswers, a)
}

// AnswerOf — ответ участника на текущее «выполнено», если он уже есть.
func (is *Issue) AnswerOf(userID int64) (Answer, bool) {
	i := slices.IndexFunc(is.answers, func(a Answer) bool { return a.UserID == userID })
	if i < 0 {
		return Answer{}, false
	}
	return is.answers[i], true
}

// ConfirmedCount — сколько участников подтвердили текущее «выполнено».
func (is *Issue) ConfirmedCount() int {
	n := 0
	for _, a := range is.answers {
		if a.Fixed {
			n++
		}
	}
	return n
}

// AnswerUntil — до какого момента можно ответить на «выполнено»; ноль, если заявка не выполнена.
func (is *Issue) AnswerUntil() time.Time {
	if is.status != StatusDone {
		return time.Time{}
	}
	return is.statusAt.Add(ConfirmWindow)
}

// Answers — ответы на текущее «выполнено».
func (is *Issue) Answers() []Answer { return slices.Clone(is.answers) }

// NewAnswers — ответы, которых ещё нет в хранилище (хранилище пишет их без дублей).
func (is *Issue) NewAnswers() []Answer { return slices.Clone(is.newAnswers) }

// ReopenedAt — когда жители в последний раз вернули заявку в работу; ноль, если не возвращали.
func (is *Issue) ReopenedAt() time.Time { return is.reopenedAt }
