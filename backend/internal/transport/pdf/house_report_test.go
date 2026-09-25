package pdf

import (
	"bytes"
	"testing"
	"time"

	"dommax/internal/domain/issue"
)

func TestHouseReportIsPDF(t *testing.T) {
	at := func(day int) time.Time { return time.Date(2026, 9, day, 10, 0, 0, 0, msk) }
	doc := HouseReportDocument{
		Address:        "Ореховый бульвар, 17к2",
		District:       "Зябликово",
		Organization:   "ГБУ «Жилищник района Зябликово»",
		Dispatcher:     "+7 495 539-53-53",
		GeneratedAt:    at(25),
		TotalIssues:    2,
		OpenIssues:     1,
		DoneIssues:     1,
		OverdueIssues:  0,
		TotalReporters: 8,
		Issues: []HouseReportIssue{
			{
				Number:       101,
				Title:        "Не работает лифт во втором подъезде",
				Category:     "Лифт",
				Status:       issue.StatusDone,
				CreatedAt:    at(18),
				Deadline:     at(19),
				Participants: 6,
				Comment:      "Заменили трос",
			},
			{
				Number:       102,
				Title:        "Не горит свет на 3 этаже",
				Category:     "Освещение",
				Status:       issue.StatusInProgress,
				CreatedAt:    at(24),
				Deadline:     at(26),
				Participants: 2,
				Comment:      "Мастер на месте",
			},
		},
	}

	out, err := HouseReport(doc)
	if err != nil {
		t.Fatalf("HouseReport: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("output does not have %PDF- header")
	}
	if len(out) < 1000 {
		t.Fatalf("PDF too small: %d bytes", len(out))
	}
}

func TestHouseReportEmptyIssues(t *testing.T) {
	doc := HouseReportDocument{
		Address:      "Тверская улица, 12",
		Organization: "ГБУ «Жилищник района Тверской»",
		GeneratedAt:  time.Now(),
	}
	out, err := HouseReport(doc)
	if err != nil {
		t.Fatalf("HouseReport with empty issues: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("not a PDF")
	}
}
