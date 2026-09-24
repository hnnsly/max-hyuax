// Пакет cards — живые карточки заявок в чате с ботом: кому и что отправить после
// изменения заявки (Plan) и доставка одного уведомления из очереди (Deliver).
package cards

import (
	"context"
	"errors"
	"slices"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
)

// Plan превращает события заявки в уведомления. Уведомляются только участники заявки
// (требование MAX о запрете рассылок): карточка обновляется при любом изменении,
// отдельное сообщение приходит только при закрытии заявки.
func Plan(events []issue.Event, participants []issue.Participant) []app.Notification {
	if len(events) == 0 {
		return nil
	}
	issueID := events[0].IssueID
	var out []app.Notification
	add := func(kind app.NotificationKind, userID int64) {
		n := app.Notification{Kind: kind, IssueID: issueID, UserID: userID}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	var closed, overdue bool
	for _, e := range events {
		switch e.Kind {
		case issue.EventCreated:
			add(app.NotifyCard, e.UserID)
		case issue.EventJoined, issue.EventStatusChanged, issue.EventOverdue:
			for _, p := range participants {
				add(app.NotifyCard, p.UserID)
			}
			closed = closed || (e.Kind == issue.EventStatusChanged && e.Status.Closed())
			overdue = overdue || e.Kind == issue.EventOverdue
		}
	}
	for _, notice := range []struct {
		on   bool
		kind app.NotificationKind
	}{{closed, app.NotifyFinal}, {overdue, app.NotifyOverdue}} {
		if !notice.on {
			continue
		}
		for _, p := range participants {
			add(notice.kind, p.UserID)
		}
	}
	return out
}

// Card — всё, что нужно показать в карточке заявки. Формат сообщения задаёт транспорт бота.
type Card struct {
	IssueID      string
	Number       int64
	Title        string
	Address      string
	Place        string
	Status       issue.Status
	StatusAt     time.Time
	Comment      string
	Deadline     time.Time
	Overdue      bool
	Participants int
	Responsible  string
}

// Messenger отправляет карточки в диалог с пользователем MAX.
type Messenger interface {
	// UpsertCard редактирует сообщение mid или, если mid пуст, отправляет новое; возвращает id сообщения.
	UpsertCard(ctx context.Context, maxUserID int64, mid string, c Card) (string, error)
	// Notify отправляет отдельное сообщение: итог по закрытой заявке или уведомление о просрочке.
	Notify(ctx context.Context, maxUserID int64, kind app.NotificationKind, c Card) error
}

type Service struct {
	store app.Store
	msg   Messenger
	now   func() time.Time
}

func NewService(store app.Store, msg Messenger, now func() time.Time) *Service {
	return &Service{store: store, msg: msg, now: now}
}

// Deliver отправляет одно уведомление, собирая карточку из актуального состояния заявки.
// Пользователи без аккаунта MAX (демо) пропускаются без ошибки.
func (s *Service) Deliver(ctx context.Context, n app.Notification) error {
	u, err := s.store.Users().Get(ctx, n.UserID)
	if err != nil {
		return err
	}
	if u.MaxUserID == 0 || u.Deleted() {
		return nil
	}
	c, err := s.card(ctx, n.IssueID)
	if err != nil {
		return err
	}
	if n.Kind != app.NotifyCard {
		return s.msg.Notify(ctx, u.MaxUserID, n.Kind, c)
	}

	mid, err := s.store.Outbox().CardMID(ctx, n.IssueID, n.UserID)
	if err != nil && !errors.Is(err, app.ErrNotFound) {
		return err
	}
	newMID, err := s.msg.UpsertCard(ctx, u.MaxUserID, mid, c)
	if err != nil && mid != "" {
		// Старое сообщение могли удалить вместе с историей чата: присылаем карточку заново.
		newMID, err = s.msg.UpsertCard(ctx, u.MaxUserID, "", c)
	}
	if err != nil {
		return err
	}
	if newMID != mid {
		return s.store.Outbox().SaveCardMID(ctx, n.IssueID, n.UserID, newMID)
	}
	return nil
}

func (s *Service) card(ctx context.Context, issueID string) (Card, error) {
	is, err := s.store.Issues().Get(ctx, issueID)
	if err != nil {
		return Card{}, err
	}
	c := Card{
		IssueID: is.ID(), Number: is.Number(), Title: is.Title(), Status: is.Status(), StatusAt: is.StatusAt(),
		Comment: is.StatusComment(), Deadline: is.Deadline(), Overdue: is.IsOverdue(s.now()), Participants: is.ParticipantCount(),
	}
	houses := s.store.Houses()
	h, err := houses.Get(ctx, is.HouseID())
	if err != nil {
		return Card{}, err
	}
	c.Address = h.Address
	if org, err := houses.Organization(ctx, is.ResponsibleOrgID()); err == nil {
		c.Responsible = org.Name
	}
	if is.ObjectID() != "" {
		objs, err := houses.Objects(ctx, is.HouseID())
		if err != nil {
			return Card{}, err
		}
		for _, o := range objs {
			if o.ID == is.ObjectID() {
				c.Place = o.Label
			}
		}
	}
	return c, nil
}
