package issue

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// ConfirmWindow — сколько дней после отметки «выполнено» жители могут подтвердить ремонт или вернуть заявку.
	ConfirmWindow = 7 * 24 * time.Hour
	// MaxCommentRunes — предел комментария «не починили»: он уходит в каждую хронологию заявки.
	MaxCommentRunes = 1000
)

var (
	ErrNotDone         = errors.New("issue: repair is not marked done")
	ErrNotParticipant  = errors.New("issue: only participants answer")
	ErrWindowClosed    = errors.New("issue: confirmation window is closed")
	ErrAlreadyAnswered = errors.New("issue: user already answered")
	ErrCommentRequired = errors.New("issue: comment required to reopen")
	ErrNotConfirmed    = errors.New("issue: only a confirmed repair is rated")
	ErrAlreadyRated    = errors.New("issue: repair already rated")
)

// Answer — ответ участника на «выполнено»: починили или нет. DoneAt — к какой отметке «выполнено»
// относится ответ: после возврата в работу и нового «выполнено» жители отвечают заново.
// Stars — оценка ремонта 1–5 после «Починили»; 0 — житель не оценивал (ADR-022).
type Answer struct {
	UserID int64
	Fixed  bool
	DoneAt time.Time
	At     time.Time
	Stars  int
}

// Rate — участник, подтвердивший ремонт, оценивает его от 1 до 5 звёзд; оценка одна на ремонт.
// Из оценок складывается рейтинг УК района.
func (is *Issue) Rate(userID int64, stars int, at time.Time) error {
	if stars < 1 || stars > 5 {
		return fmt.Errorf("%w: stars must be from 1 to 5", ErrInvalid)
	}
	if !is.HasParticipant(userID) {
		return ErrNotParticipant
	}
	i := slices.IndexFunc(is.answers, func(a Answer) bool { return a.UserID == userID })
	switch {
	case i < 0 || !is.answers[i].Fixed:
		return ErrNotConfirmed
	case is.answers[i].Stars != 0:
		return ErrAlreadyRated
	case at.After(is.statusAt.Add(ConfirmWindow)):
		return ErrWindowClosed
	}
	is.answers[i].Stars = stars
	is.newRatings = append(is.newRatings, is.answers[i])
	return nil
}

// NewRatings — оценки, которых ещё нет в хранилище.
func (is *Issue) NewRatings() []Answer { return slices.Clone(is.newRatings) }

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
	if utf8.RuneCountInString(comment) > MaxCommentRunes {
		return fmt.Errorf("%w: comment is longer than %d characters", ErrInvalid, MaxCommentRunes)
	}
	if !newDeadline.After(at) {
		return fmt.Errorf("%w: new deadline must be after reopening", ErrInvalid)
	}
	is.answer(Answer{UserID: userID, Fixed: false, DoneAt: is.statusAt, At: at})
	is.status, is.statusAt, is.statusComment = StatusInProgress, at, ""
	// Если заявка уже была просрочена до отметки «выполнено», overdueAt сохраняется:
	// фиктивное закрытие не позволяет УК сбросить просрочку и лишить жителя обращения в ГЖИ.
	is.deadline, is.reopenedAt = newDeadline, at
	is.answers = nil // круг ответов закончен: следующий начнётся с нового «выполнено»
	is.record(Event{Kind: EventReopened, UserID: userID, Status: is.status, Comment: comment, At: at})
	return nil
}

// canAnswer: сначала участие, потом состояние заявки — чужому незачем знать, выполнена ли она.
func (is *Issue) canAnswer(userID int64, at time.Time) error {
	switch {
	case !is.HasParticipant(userID):
		return ErrNotParticipant
	case is.status != StatusDone:
		return ErrNotDone
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
