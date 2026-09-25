package pdf_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"dommax/internal/transport/pdf"
)

func TestStickerPDFIsValid(t *testing.T) {
	row := strings.Repeat("10", 10) + "1"
	rows := make([]string, 21)
	for i := range rows {
		rows[i] = row
	}
	raw, err := pdf.StickerPDF(pdf.StickerDocument{
		Address:         "Ореховый бульвар, 17к2",
		Organization:    "УК «Ореховый квартал»",
		PhoneDispatcher: "+7 495 000-17-02",
		Category:        "lift",
		Label:           "подъезд 2, пассажирский лифт",
		EntranceNumber:  2,
		QRCode:          "h-17k2-e2-lift",
		BotName:         "t105_hakaton_max_bot",
		QRMatrix:        strings.Join(rows, "."),
		GeneratedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("StickerPDF: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte("%PDF-")) || len(raw) < 1000 {
		t.Fatalf("unexpected pdf output: %d bytes", len(raw))
	}
}
