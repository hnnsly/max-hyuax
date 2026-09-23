// Пакет issue содержит агрегат Issue: проблему с общим имуществом дома, о которой
// жители сообщают один раз и присоединяются к ней вместо создания дублей.
package issue

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type Status string

const (
	StatusSent       Status = "sent"
	StatusAccepted   Status = "accepted"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusRejected   Status = "rejected"
)

// Closed сообщает, что работы и присоединения по заявке больше не ожидаются.
func (s Status) Closed() bool { return s == StatusDone || s == StatusRejected }

var transitions = map[Status][]Status{
	StatusSent:       {StatusAccepted, StatusInProgress, StatusRejected},
	StatusAccepted:   {StatusInProgress, StatusDone, StatusRejected},
	StatusInProgress: {StatusDone, StatusRejected},
}

var (
	ErrInvalid        = errors.New("issue: invalid data")
	ErrAlreadyJoined  = errors.New("issue: user already joined")
	ErrClosed         = errors.New("issue: issue is closed")
	ErrTransition     = errors.New("issue: status transition not allowed")
	ErrReasonRequired = errors.New("issue: rejection reason required")
)

type EventKind string

const (
	EventCreated       EventKind = "created"
	EventJoined        EventKind = "joined"
	EventStatusChanged EventKind = "status_changed"
)

// Event — доменное событие; слой приложения превращает события в записи outbox.
type Event struct {
	Kind    EventKind
	IssueID string
	UserID  int64
	Status  Status
	Comment string
	At      time.Time
}

type Participant struct {
	UserID   int64
	JoinedAt time.Time
}

type Issue struct {
	id           string
	houseID      string
	objectID     string
	category     string
	title        string
	description  string
	status       Status
	createdAt    time.Time
	deadline     time.Time
	participants []Participant
	events       []Event
}

type NewParams struct {
	ID          string
	HouseID     string
	ObjectID    string
	Category    string
	Title       string
	Description string
	ReporterID  int64
	CreatedAt   time.Time
	Deadline    time.Time
}

// New создаёт заявку; автор становится её первым участником.
func New(p NewParams) (*Issue, error) {
	switch {
	case p.ID == "", p.HouseID == "", p.ReporterID == 0:
		return nil, fmt.Errorf("%w: id, house and reporter are required", ErrInvalid)
	case strings.TrimSpace(p.Title) == "", p.Category == "":
		return nil, fmt.Errorf("%w: title and category are required", ErrInvalid)
	case !p.Deadline.After(p.CreatedAt):
		return nil, fmt.Errorf("%w: deadline must be after creation", ErrInvalid)
	}
	is := &Issue{
		id:           p.ID,
		houseID:      p.HouseID,
		objectID:     p.ObjectID,
		category:     p.Category,
		title:        strings.TrimSpace(p.Title),
		description:  strings.TrimSpace(p.Description),
		status:       StatusSent,
		createdAt:    p.CreatedAt,
		deadline:     p.Deadline,
		participants: []Participant{{UserID: p.ReporterID, JoinedAt: p.CreatedAt}},
	}
	is.record(Event{Kind: EventCreated, UserID: p.ReporterID, Status: StatusSent, At: p.CreatedAt})
	return is, nil
}

// Join добавляет соседа с той же проблемой («это и у меня»).
func (is *Issue) Join(userID int64, at time.Time) error {
	if is.status.Closed() {
		return ErrClosed
	}
	if is.HasParticipant(userID) {
		return ErrAlreadyJoined
	}
	is.participants = append(is.participants, Participant{UserID: userID, JoinedAt: at})
	is.record(Event{Kind: EventJoined, UserID: userID, Status: is.status, At: at})
	return nil
}

// ChangeStatus переводит заявку по жизненному циклу; отказ обязан содержать причину для жителей.
func (is *Issue) ChangeStatus(to Status, comment string, at time.Time) error {
	if !slices.Contains(transitions[is.status], to) {
		return fmt.Errorf("%w: %s -> %s", ErrTransition, is.status, to)
	}
	comment = strings.TrimSpace(comment)
	if to == StatusRejected && comment == "" {
		return ErrReasonRequired
	}
	is.status = to
	is.record(Event{Kind: EventStatusChanged, Status: to, Comment: comment, At: at})
	return nil
}

// IsOverdue сообщает, что срок ответа прошёл, а заявка всё ещё открыта.
func (is *Issue) IsOverdue(now time.Time) bool {
	return !is.status.Closed() && now.After(is.deadline)
}

func (is *Issue) HasParticipant(userID int64) bool {
	return slices.ContainsFunc(is.participants, func(p Participant) bool { return p.UserID == userID })
}

func (is *Issue) ParticipantCount() int { return len(is.participants) }

// PullEvents возвращает накопленные доменные события и очищает их.
func (is *Issue) PullEvents() []Event {
	events := is.events
	is.events = nil
	return events
}

func (is *Issue) record(e Event) {
	e.IssueID = is.id
	is.events = append(is.events, e)
}

func (is *Issue) ID() string                  { return is.id }
func (is *Issue) HouseID() string             { return is.houseID }
func (is *Issue) ObjectID() string            { return is.objectID }
func (is *Issue) Category() string            { return is.category }
func (is *Issue) Title() string               { return is.title }
func (is *Issue) Description() string         { return is.description }
func (is *Issue) Status() Status              { return is.status }
func (is *Issue) CreatedAt() time.Time        { return is.createdAt }
func (is *Issue) Deadline() time.Time         { return is.deadline }
func (is *Issue) Participants() []Participant { return slices.Clone(is.participants) }

// Restore восстанавливает агрегат из хранилища без записи событий.
func Restore(p NewParams, status Status, participants []Participant) *Issue {
	return &Issue{
		id:           p.ID,
		houseID:      p.HouseID,
		objectID:     p.ObjectID,
		category:     p.Category,
		title:        p.Title,
		description:  p.Description,
		status:       status,
		createdAt:    p.CreatedAt,
		deadline:     p.Deadline,
		participants: slices.Clone(participants),
	}
}
