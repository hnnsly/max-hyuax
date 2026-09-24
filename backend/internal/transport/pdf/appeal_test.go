package pdf

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"dommax/internal/app/appeal"
	"dommax/internal/domain/issue"
)

var msk = time.FixedZone("MSK", 3*60*60)

func sample() appeal.Document {
	at := func(day, hour int) time.Time { return time.Date(2026, 9, day, hour, 0, 0, 0, msk) }
	return appeal.Document{
		Number: 142, Address: "Ореховый бульвар, 17к2", Place: "подъезд 2, пассажирский лифт",
		Category: "Лифт", Title: "Лифт не работает", Description: "Кабина не приходит на вызов",
		Organization: "УК «Ореховый квартал»", CreatedAt: at(17, 9), Deadline: at(18, 23),
		Participants: 13, Basis: "Правила управления МКД (ПП № 416)",
		Events: []issue.Event{
			{Kind: issue.EventCreated, Status: issue.StatusSent, At: at(17, 9)},
			{Kind: issue.EventJoined, Status: issue.StatusSent, At: at(17, 10)},
			{Kind: issue.EventJoined, Status: issue.StatusSent, At: at(17, 11)},
			{Kind: issue.EventStatusChanged, Status: issue.StatusAccepted, Comment: "Мастер приедет", At: at(17, 12)},
			{Kind: issue.EventOverdue, Status: issue.StatusAccepted, At: at(19, 0)},
		},
		GeneratedAt: at(20, 10),
	}
}

func TestTimelineLinesCollapseJoinsWithoutNames(t *testing.T) {
	got := timelineLines(sample().Events)
	want := []string{
		"17.09.2026 09:00  Житель сообщил о проблеме",
		"17.09.2026 11:00  Присоединились ещё 2 соседа",
		"17.09.2026 12:00  УК приняла заявку. Комментарий УК: «Мастер приедет»",
		"19.09.2026 00:00  Срок ответа истёк",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("timeline:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestAppealIsPDF(t *testing.T) {
	out, err := Appeal(sample())
	if err != nil {
		t.Fatalf("Appeal: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) || !bytes.Contains(out[len(out)-32:], []byte("%%EOF")) {
		t.Fatalf("not a PDF: %q ... %q", out[:8], out[len(out)-16:])
	}
	if n := countPages(out); n != 1 {
		t.Fatalf("pages = %d, want 1", n)
	}
}

// countPages считает объекты страниц: словари объектов в PDF не сжимаются.
func countPages(pdf []byte) int {
	return len(regexp.MustCompile(`/Type\s*/Page[^s]`).FindAll(pdf, -1))
}

// Длинное описание и хронология не обрезаются: документ переходит на вторую страницу.
func TestLongAppealBreaksPages(t *testing.T) {
	d := sample()
	d.Description = strings.Repeat("Кабина не приходит на вызов, горит индикатор перегрузки. ", 60)
	for range 40 {
		d.Events = append(d.Events, issue.Event{Kind: issue.EventStatusChanged, Status: issue.StatusInProgress, Comment: "Ждём запчасть", At: d.GeneratedAt})
	}
	short, err := Appeal(sample())
	if err != nil {
		t.Fatal(err)
	}
	long, err := Appeal(d)
	if err != nil {
		t.Fatalf("Appeal long: %v", err)
	}
	if countPages(long) <= countPages(short) {
		t.Fatalf("pages: long %d, short %d", countPages(long), countPages(short))
	}
}

func TestAppealWithoutOptionalFields(t *testing.T) {
	d := sample()
	d.Place, d.Description, d.Events = "", "", nil
	if _, err := Appeal(d); err != nil {
		t.Fatalf("Appeal minimal: %v", err)
	}
}
