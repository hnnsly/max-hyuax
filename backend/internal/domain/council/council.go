// Пакет council — совет дома: предложения жителей председателю и опросы без юридической силы (ADR-017).
package council

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MinTextRunes     = 10   // предложение короче — не предложение, а «привет»
	MaxTextRunes     = 1000 // и предложение, и ответ председателя
	MaxOptions       = 5
	MaxOptionRunes   = 100
	MaxQuestionRunes = 300
	MaxPollDays      = 14
)

var (
	ErrInvalid         = errors.New("invalid council input")
	ErrAlreadyAnswered = errors.New("proposal already answered")
	ErrPollClosed      = errors.New("poll closed")
	ErrAlreadyVoted    = errors.New("already voted")
)

type Status string

const (
	StatusNew      Status = "new"
	StatusAccepted Status = "accepted" // председатель взял в работу
	StatusDeclined Status = "declined" // председатель отклонил с ответом
)

// Proposal — предложение жителя председателю совета своего дома. Автора председатель не видит.
type Proposal struct {
	ID         string
	HouseID    string
	AuthorID   int64
	Text       string
	Status     Status
	Answer     string
	CreatedAt  time.Time
	AnsweredAt time.Time
}

func NewProposal(id, houseID string, authorID int64, text string, at time.Time) (Proposal, error) {
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n < MinTextRunes || n > MaxTextRunes {
		return Proposal{}, fmt.Errorf("%w: proposal text must be %d..%d characters", ErrInvalid, MinTextRunes, MaxTextRunes)
	}
	return Proposal{ID: id, HouseID: houseID, AuthorID: authorID, Text: text, Status: StatusNew, CreatedAt: at}, nil
}

// Reply — ответ председателя. Отклонить можно только с объяснением; ответить можно один раз.
func (p *Proposal) Reply(status Status, answer string, at time.Time) error {
	if p.Status != StatusNew {
		return ErrAlreadyAnswered
	}
	answer = strings.TrimSpace(answer)
	switch {
	case status != StatusAccepted && status != StatusDeclined:
		return fmt.Errorf("%w: status must be accepted or declined", ErrInvalid)
	case status == StatusDeclined && answer == "":
		return fmt.Errorf("%w: answer is required to decline", ErrInvalid)
	case utf8.RuneCountInString(answer) > MaxTextRunes:
		return fmt.Errorf("%w: answer is longer than %d characters", ErrInvalid, MaxTextRunes)
	}
	p.Status, p.Answer, p.AnsweredAt = status, answer, at
	return nil
}

// Poll — опрос жителей дома без юридической силы: не заменяет общее собрание собственников.
type Poll struct {
	ID         string
	HouseID    string
	ProposalID string // пусто — опрос не из предложения
	Question   string
	Options    []string
	CreatedAt  time.Time
	ClosesAt   time.Time
}

func NewPoll(id, houseID, proposalID, question string, options []string, days int, at time.Time) (Poll, error) {
	question = strings.TrimSpace(question)
	if n := utf8.RuneCountInString(question); n < 5 || n > MaxQuestionRunes {
		return Poll{}, fmt.Errorf("%w: question must be 5..%d characters", ErrInvalid, MaxQuestionRunes)
	}
	if len(options) < 2 || len(options) > MaxOptions {
		return Poll{}, fmt.Errorf("%w: poll needs 2..%d options", ErrInvalid, MaxOptions)
	}
	opts := make([]string, len(options))
	for i, o := range options {
		opts[i] = strings.TrimSpace(o)
		if n := utf8.RuneCountInString(opts[i]); n == 0 || n > MaxOptionRunes || slices.Contains(opts[:i], opts[i]) {
			return Poll{}, fmt.Errorf("%w: options must be non-empty, unique, up to %d characters", ErrInvalid, MaxOptionRunes)
		}
	}
	if days < 1 || days > MaxPollDays {
		return Poll{}, fmt.Errorf("%w: poll lasts 1..%d days", ErrInvalid, MaxPollDays)
	}
	return Poll{ID: id, HouseID: houseID, ProposalID: proposalID, Question: question, Options: opts,
		CreatedAt: at, ClosesAt: at.AddDate(0, 0, days)}, nil
}

func (p Poll) Open(now time.Time) bool { return now.Before(p.ClosesAt) }

// CheckVote — можно ли сейчас голосовать за вариант option (номер с нуля).
func (p Poll) CheckVote(option int, now time.Time) error {
	if !p.Open(now) {
		return ErrPollClosed
	}
	if option < 0 || option >= len(p.Options) {
		return fmt.Errorf("%w: no option %d", ErrInvalid, option)
	}
	return nil
}
