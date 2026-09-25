package council_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"dommax/internal/domain/council"
)

var t0 = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

func TestNewProposalChecksText(t *testing.T) {
	p, err := council.NewProposal("p1", "h-1", 7, "  Поставить велопарковку у второго подъезда  ", t0)
	if err != nil || p.Text != "Поставить велопарковку у второго подъезда" || p.Status != council.StatusNew {
		t.Fatalf("proposal = %+v, err = %v", p, err)
	}
	for _, text := range []string{"", "коротко", strings.Repeat("я", council.MaxTextRunes+1)} {
		if _, err := council.NewProposal("p2", "h-1", 7, text, t0); !errors.Is(err, council.ErrInvalid) {
			t.Errorf("NewProposal(%d runes) err = %v", len([]rune(text)), err)
		}
	}
}

func TestReply(t *testing.T) {
	p, _ := council.NewProposal("p1", "h-1", 7, "Покрасить лавочки у площадки", t0)
	if err := p.Reply(council.StatusDeclined, " ", t0); !errors.Is(err, council.ErrInvalid) {
		t.Fatalf("decline without answer err = %v", err)
	}
	if err := p.Reply("maybe", "", t0); !errors.Is(err, council.ErrInvalid) {
		t.Fatalf("unknown status err = %v", err)
	}
	if err := p.Reply(council.StatusAccepted, "", t0.Add(time.Hour)); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if p.Status != council.StatusAccepted || !p.AnsweredAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("proposal = %+v", p)
	}
	if err := p.Reply(council.StatusDeclined, "Передумал", t0); !errors.Is(err, council.ErrAlreadyAnswered) {
		t.Fatalf("second reply err = %v", err)
	}
}

func TestNewPollAndVote(t *testing.T) {
	poll, err := council.NewPoll("q1", "h-1", "p1", "Ставим шлагбаум во дворе?", []string{" За ", "Против", "Воздержусь"}, 7, t0)
	if err != nil || poll.Options[0] != "За" || !poll.ClosesAt.Equal(t0.AddDate(0, 0, 7)) {
		t.Fatalf("poll = %+v, err = %v", poll, err)
	}
	if !poll.Open(t0) || poll.Open(t0.AddDate(0, 0, 7)) {
		t.Fatal("poll must be open until ClosesAt")
	}
	if err := poll.CheckVote(2, t0); err != nil {
		t.Fatalf("vote: %v", err)
	}
	if err := poll.CheckVote(3, t0); !errors.Is(err, council.ErrInvalid) {
		t.Fatalf("vote for missing option err = %v", err)
	}
	if err := poll.CheckVote(0, t0.AddDate(0, 0, 8)); !errors.Is(err, council.ErrPollClosed) {
		t.Fatalf("vote after close err = %v", err)
	}

	bad := [][]string{{"За"}, {"За", "За"}, {"За", " "}, {"1", "2", "3", "4", "5", "6"}, {"За", strings.Repeat("а", 101)}}
	for _, opts := range bad {
		if _, err := council.NewPoll("q2", "h-1", "", "Вопрос к дому?", opts, 7, t0); !errors.Is(err, council.ErrInvalid) {
			t.Errorf("options %q err = %v", opts, err)
		}
	}
	if _, err := council.NewPoll("q3", "h-1", "", "?", []string{"За", "Против"}, 7, t0); !errors.Is(err, council.ErrInvalid) {
		t.Error("short question accepted")
	}
	for _, days := range []int{0, 15} {
		if _, err := council.NewPoll("q4", "h-1", "", "Вопрос к дому?", []string{"За", "Против"}, days, t0); !errors.Is(err, council.ErrInvalid) {
			t.Errorf("days %d accepted", days)
		}
	}
}
