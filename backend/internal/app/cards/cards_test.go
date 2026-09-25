package cards_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	"dommax/internal/app/cards"
	"dommax/internal/domain/council"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

var at = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

func parts(ids ...int64) []issue.Participant {
	var out []issue.Participant
	for _, id := range ids {
		out = append(out, issue.Participant{UserID: id})
	}
	return out
}

func card(id int64) app.Notification {
	return app.Notification{Kind: app.NotifyCard, IssueID: "i1", UserID: id}
}
func final(id int64) app.Notification {
	return app.Notification{Kind: app.NotifyFinal, IssueID: "i1", UserID: id}
}
func status(id int64) app.Notification {
	return app.Notification{Kind: app.NotifyStatus, IssueID: "i1", UserID: id}
}
func overdue(id int64) app.Notification {
	return app.Notification{Kind: app.NotifyOverdue, IssueID: "i1", UserID: id}
}

func TestPlanNotifiesOnlyParticipants(t *testing.T) {
	cases := []struct {
		name   string
		events []issue.Event
		people []issue.Participant
		want   []app.Notification
	}{
		{"created: card to reporter", []issue.Event{{Kind: issue.EventCreated, IssueID: "i1", UserID: 1}}, parts(1), []app.Notification{card(1)}},
		{"joined: counter refresh for all", []issue.Event{{Kind: issue.EventJoined, IssueID: "i1", UserID: 2}}, parts(1, 2), []app.Notification{card(1), card(2)}},
		// «В работе» — карточка и отдельное сообщение: правка карточки телефон не подсвечивает.
		{"status: cards and status messages", []issue.Event{{Kind: issue.EventStatusChanged, IssueID: "i1", Status: issue.StatusInProgress}}, parts(1, 2),
			[]app.Notification{card(1), card(2), status(1), status(2)}},
		{"done: cards and final messages", []issue.Event{{Kind: issue.EventStatusChanged, IssueID: "i1", Status: issue.StatusDone}}, parts(1, 2),
			[]app.Notification{card(1), card(2), final(1), final(2)}},
		{"overdue: cards and overdue messages", []issue.Event{{Kind: issue.EventOverdue, IssueID: "i1", Status: issue.StatusAccepted}}, parts(1, 2),
			[]app.Notification{card(1), card(2), overdue(1), overdue(2)}},
		{"several events are deduplicated", []issue.Event{
			{Kind: issue.EventJoined, IssueID: "i1", UserID: 2},
			{Kind: issue.EventStatusChanged, IssueID: "i1", Status: issue.StatusAccepted},
		}, parts(1, 2), []app.Notification{card(1), card(2), status(1), status(2)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cards.Plan(tc.events, tc.people); !slices.Equal(got, tc.want) {
				t.Fatalf("plan = %+v, want %+v", got, tc.want)
			}
		})
	}
}

type sent struct {
	maxUser int64
	mid     string
	card    cards.Card
	notice  app.NotificationKind // пусто — карточка, иначе отдельное сообщение этого вида
}

type fakeMessenger struct {
	council []string // «вид:id предложения» отправленных сообщений совета
	calls   []sent
	editErr error
}

func (f *fakeMessenger) UpsertCard(_ context.Context, maxUser int64, mid string, c cards.Card) (string, error) {
	f.calls = append(f.calls, sent{maxUser: maxUser, mid: mid, card: c})
	if mid != "" && f.editErr != nil {
		return "", f.editErr
	}
	if mid == "" {
		return "mid-new", nil
	}
	return mid, nil
}

func (f *fakeMessenger) NotifyCouncil(_ context.Context, maxUser int64, kind app.NotificationKind, p council.Proposal) error {
	f.council = append(f.council, string(kind)+":"+p.ID)
	return nil
}

func (f *fakeMessenger) Notify(_ context.Context, maxUser int64, kind app.NotificationKind, c cards.Card) error {
	f.calls = append(f.calls, sent{maxUser: maxUser, card: c, notice: kind})
	return nil
}

func setup(t *testing.T) (*apptest.MemStore, *fakeMessenger, *cards.Service, *issue.Issue) {
	t.Helper()
	s := apptest.New()
	s.Orgs["org-1"] = house.Organization{ID: "org-1", Name: "УК «Ореховый квартал»"}
	s.HouseMap["h-1"] = house.House{ID: "h-1", Address: "Ореховый бульвар, 17к2", OrganizationID: "org-1"}
	s.Objects = []house.AssetObject{{ID: "o-1", HouseID: "h-1", Label: "подъезд 2, пассажирский лифт"}}
	s.AddUser(user.User{ID: 1, MaxUserID: 5001})
	s.AddUser(user.User{ID: 2}) // демо-пользователь без аккаунта MAX
	is, err := issue.New(issue.NewParams{ID: "i1", HouseID: "h-1", ObjectID: "o-1", Category: "lift", Title: "Лифт не работает",
		ResponsibleOrgID: "org-1", ReporterID: 1, CreatedAt: at, Deadline: at.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Issues().Create(t.Context(), is); err != nil {
		t.Fatal(err)
	}
	m := &fakeMessenger{}
	return s, m, cards.NewService(s, m, func() time.Time { return at }), is
}

func TestDeliverSendsThenEditsTheSameCard(t *testing.T) {
	s, m, svc, _ := setup(t)
	if err := svc.Deliver(t.Context(), card(1)); err != nil {
		t.Fatal(err)
	}
	if err := svc.Deliver(t.Context(), card(1)); err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 2 || m.calls[0].mid != "" || m.calls[1].mid != "mid-new" || m.calls[0].maxUser != 5001 {
		t.Fatalf("calls = %+v", m.calls)
	}
	c := m.calls[0].card
	if c.Number == 0 || c.Place != "подъезд 2, пассажирский лифт" || c.Responsible != "УК «Ореховый квартал»" || c.Participants != 1 {
		t.Fatalf("card = %+v", c)
	}
	if mid, _ := s.Outbox().CardMID(t.Context(), "i1", 1); mid != "mid-new" {
		t.Fatalf("stored mid = %q", mid)
	}
}

func TestDeliverResendsWhenEditFails(t *testing.T) {
	s, m, svc, _ := setup(t)
	_ = s.Outbox().SaveCardMID(t.Context(), "i1", 1, "mid-old")
	m.editErr = errors.New("message not found")
	if err := svc.Deliver(t.Context(), card(1)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(m.calls) != 2 || m.calls[1].mid != "" {
		t.Fatalf("calls = %+v, want edit then fresh send", m.calls)
	}
}

func TestDeliverSkipsUsersWithoutMax(t *testing.T) {
	_, m, svc, _ := setup(t)
	if err := svc.Deliver(t.Context(), card(2)); err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 0 {
		t.Fatalf("calls = %+v", m.calls)
	}
}

func TestDeliverNotices(t *testing.T) {
	_, m, svc, _ := setup(t)
	for _, n := range []app.Notification{final(1), overdue(1)} {
		if err := svc.Deliver(t.Context(), n); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.calls) != 2 || m.calls[0].notice != app.NotifyFinal || m.calls[1].notice != app.NotifyOverdue {
		t.Fatalf("calls = %+v", m.calls)
	}
}
