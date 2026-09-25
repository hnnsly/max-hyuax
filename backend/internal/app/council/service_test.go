package council_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/apptest"
	appcouncil "dommax/internal/app/council"
	"dommax/internal/domain/council"
	"dommax/internal/domain/user"
)

var now = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

func setup() (*appcouncil.Service, *time.Time) {
	clock := now
	ids := 0
	svc := appcouncil.NewService(apptest.New(), appcouncil.Config{
		Now:            func() time.Time { return clock },
		NewID:          func() string { ids++; return "id-" + strconv.Itoa(ids) },
		ConsentVersion: "v1",
	})
	return svc, &clock
}

func resident(id int64, house string) user.User {
	return user.User{ID: id, Role: user.RoleResident, HouseID: house, ConsentVersion: "v1"}
}

func chairman(id int64, house string) user.User {
	u := resident(id, house)
	u.ChairmanHouseID = house
	return u
}

func TestProposalFlow(t *testing.T) {
	svc, _ := setup()
	ctx := t.Context()
	anna, nina := resident(1, "h-1"), chairman(2, "h-1")

	p, err := svc.Propose(ctx, anna, "Поставить велопарковку у второго подъезда")
	if err != nil || p.HouseID != "h-1" || p.AuthorID != 1 {
		t.Fatalf("Propose = %+v, %v", p, err)
	}
	noConsent := anna
	noConsent.ConsentVersion = ""
	if _, err := svc.Propose(ctx, noConsent, "Поставить велопарковку"); !errors.Is(err, app.ErrConsentRequired) {
		t.Fatalf("without consent err = %v", err)
	}
	for _, u := range []user.User{{ID: 9, Role: user.RoleOperator, HouseID: "h-1"}, resident(3, "")} {
		if _, err := svc.Propose(ctx, u, "Поставить велопарковку"); !errors.Is(err, app.ErrForbidden) {
			t.Fatalf("Propose by %+v err = %v", u, err)
		}
	}

	if _, err := svc.Folder(ctx, anna); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("Folder by resident err = %v", err)
	}
	folder, err := svc.Folder(ctx, nina)
	if err != nil || len(folder) != 1 {
		t.Fatalf("Folder = %+v, %v", folder, err)
	}
	// Председатель другого дома предложение не видит: для него его нет.
	if _, err := svc.Reply(ctx, chairman(4, "h-2"), p.ID, council.StatusAccepted, ""); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("Reply by other chairman err = %v", err)
	}
	ninaNoConsent := nina
	ninaNoConsent.ConsentVersion = ""
	if _, err := svc.Reply(ctx, ninaNoConsent, p.ID, council.StatusAccepted, ""); !errors.Is(err, app.ErrConsentRequired) {
		t.Fatalf("Reply without consent err = %v", err)
	}
	if _, err := svc.CreatePoll(ctx, ninaNoConsent, appcouncil.PollInput{Question: "Вопрос к дому?", Options: []string{"За", "Против"}, Days: 7}); !errors.Is(err, app.ErrConsentRequired) {
		t.Fatalf("CreatePoll without consent err = %v", err)
	}
	got, err := svc.Reply(ctx, nina, p.ID, council.StatusDeclined, "Места нет, предложу УК стойку во дворе")
	if err != nil || got.Status != council.StatusDeclined || !got.AnsweredAt.Equal(now) {
		t.Fatalf("Reply = %+v, %v", got, err)
	}
	if _, err := svc.Reply(ctx, nina, p.ID, council.StatusAccepted, ""); !errors.Is(err, council.ErrAlreadyAnswered) {
		t.Fatalf("second Reply err = %v", err)
	}
	mine, err := svc.MyProposals(ctx, anna)
	if err != nil || len(mine) != 1 || mine[0].Answer == "" {
		t.Fatalf("MyProposals = %+v, %v", mine, err)
	}
}

func TestPollFlow(t *testing.T) {
	svc, clock := setup()
	ctx := t.Context()
	anna, sergey, nina := resident(1, "h-1"), resident(3, "h-1"), chairman(2, "h-1")
	p, _ := svc.Propose(ctx, anna, "Поставить шлагбаум на въезде во двор")

	in := appcouncil.PollInput{ProposalID: p.ID, Question: "Ставим шлагбаум?", Options: []string{"За", "Против"}, Days: 7}
	if _, err := svc.CreatePoll(ctx, anna, in); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("CreatePoll by resident err = %v", err)
	}
	if _, err := svc.CreatePoll(ctx, chairman(4, "h-2"), in); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("CreatePoll from other house proposal err = %v", err)
	}
	poll, err := svc.CreatePoll(ctx, nina, in)
	if err != nil || poll.HouseID != "h-1" || poll.Mine != -1 || len(poll.Votes) != 2 || !poll.Open {
		t.Fatalf("CreatePoll = %+v, %v", poll, err)
	}

	if _, err := svc.CreatePoll(ctx, nina, in); !errors.Is(err, council.ErrPollExists) {
		t.Fatalf("second poll for the same proposal err = %v", err)
	}

	v, err := svc.Vote(ctx, anna, poll.ID, 0)
	if err != nil || v.Votes[0] != 1 || v.Mine != 0 {
		t.Fatalf("Vote = %+v, %v", v, err)
	}
	if _, err := svc.Vote(ctx, anna, poll.ID, 1); !errors.Is(err, council.ErrAlreadyVoted) {
		t.Fatalf("second vote err = %v", err)
	}
	if _, err := svc.Vote(ctx, resident(5, "h-2"), poll.ID, 1); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("vote from other house err = %v", err)
	}
	if _, err := svc.Vote(ctx, sergey, poll.ID, 7); !errors.Is(err, council.ErrInvalid) {
		t.Fatalf("vote for missing option err = %v", err)
	}
	list, err := svc.Polls(ctx, sergey)
	if err != nil || len(list) != 1 || list[0].Votes[0] != 1 || list[0].Mine != -1 {
		t.Fatalf("Polls = %+v, %v", list, err)
	}

	*clock = now.AddDate(0, 0, 8)
	if _, err := svc.Vote(ctx, sergey, poll.ID, 1); !errors.Is(err, council.ErrPollClosed) {
		t.Fatalf("vote after close err = %v", err)
	}
	if list, _ := svc.Polls(ctx, sergey); list[0].Open {
		t.Fatal("closed poll is shown as open")
	}
}
