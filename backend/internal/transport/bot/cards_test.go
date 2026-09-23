package bot_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"dommax/internal/app/cards"
	"dommax/internal/domain/issue"
	"dommax/internal/storage/maxapi"
	"dommax/internal/transport/bot"
)

var msk = time.FixedZone("MSK", 3*60*60)

func sampleCard() cards.Card {
	return cards.Card{
		IssueID: "0190a000-0000-7000-8000-000000000001", Number: 142, Title: "Лифт *не* работает",
		Address: "Ореховый бульвар, 17к2", Place: "подъезд 2, пассажирский лифт",
		Status: issue.StatusAccepted, StatusAt: time.Date(2026, 9, 17, 12, 38, 0, 0, msk),
		Comment: "Мастер приедет завтра", Deadline: time.Date(2026, 9, 19, 23, 59, 59, 0, msk),
		Participants: 13, Responsible: "УК «Ореховый квартал»",
	}
}

func TestRenderCardFollowsDesign(t *testing.T) {
	m := bot.RenderCard(sampleCard(), "t105_hakaton_max_bot")
	for _, want := range []string{
		"**Заявка № 142**",
		"Лифт не работает", // разметка из текста жителя вычищается
		"Подъезд 2, пассажирский лифт",
		"Статус: принята, отвечает УК «Ореховый квартал»",
		"Срок: до 19 сентября",
		"Сообщили соседи: 13",
		"Комментарий УК: Мастер приедет завтра",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("text has no %q:\n%s", want, m.Text)
		}
	}
	if strings.ContainsAny(m.Text, "—–") {
		t.Errorf("no dashes allowed: %s", m.Text)
	}
	if m.Format != "markdown" {
		t.Errorf("format = %q", m.Format)
	}
	btns := m.Attachments[0].Payload.(maxapi.KeyboardPayload).Buttons[0]
	if btns[0].Type != "open_app" || btns[0].Payload != "i_0190a000-0000-7000-8000-000000000001" || btns[1].Type != "link" ||
		!strings.HasPrefix(btns[1].URL, "https://max.ru/:share?text=") {
		t.Errorf("buttons = %+v", btns)
	}
}

func TestRenderCardOverdueAndClosed(t *testing.T) {
	c := sampleCard()
	c.Overdue = true
	if m := bot.RenderCard(c, "b"); !strings.Contains(m.Text, "Срок: истёк 19 сентября") {
		t.Errorf("overdue text:\n%s", m.Text)
	}
	c.Overdue, c.Status = false, issue.StatusDone
	m := bot.RenderCard(c, "b")
	if !strings.Contains(m.Text, "Статус: выполнена 17 сентября") || strings.Contains(m.Text, "Срок:") {
		t.Errorf("done text:\n%s", m.Text)
	}
}

type fakeCardAPI struct {
	sent   []maxapi.Target
	edited []string
}

func (f *fakeCardAPI) Send(_ context.Context, to maxapi.Target, _ maxapi.NewMessage) (maxapi.Message, error) {
	f.sent = append(f.sent, to)
	return maxapi.Message{Body: maxapi.MessageBody{MID: "mid-1"}}, nil
}

func (f *fakeCardAPI) Edit(_ context.Context, mid string, _ maxapi.NewMessage) error {
	f.edited = append(f.edited, mid)
	return nil
}

func TestCardSenderSendsThenEdits(t *testing.T) {
	api := &fakeCardAPI{}
	s := bot.NewCardSender(api, "b")
	mid, err := s.UpsertCard(t.Context(), 5001, "", sampleCard())
	if err != nil || mid != "mid-1" || len(api.sent) != 1 || api.sent[0] != maxapi.ToUser(5001) {
		t.Fatalf("send: mid=%q err=%v sent=%v", mid, err, api.sent)
	}
	mid, err = s.UpsertCard(t.Context(), 5001, "mid-1", sampleCard())
	if err != nil || mid != "mid-1" || len(api.edited) != 1 {
		t.Fatalf("edit: mid=%q err=%v edited=%v", mid, err, api.edited)
	}
	if err := s.SendFinal(t.Context(), 5001, sampleCard()); err != nil || len(api.sent) != 2 {
		t.Fatalf("final: err=%v sent=%v", err, api.sent)
	}
}
