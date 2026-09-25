package pdf

import (
	"fmt"
	"strings"
	"time"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// StickerDocument — данные для генерации печатной QR-наклейки объекта формата A6.
type StickerDocument struct {
	Address         string
	Organization    string
	PhoneDispatcher string
	Category        string
	Label           string
	EntranceNumber  int
	QRCode          string
	BotName         string
	QRMatrix        string
	GeneratedAt     time.Time
}

// StickerPDF генерирует печатную наклейку формата A6 с адресной табличкой, QR-кодом объекта
// и телефоном аварийно-диспетчерской службы УК.
func StickerPDF(d StickerDocument) ([]byte, error) {
	p := &gopdf.GoPdf{}
	// Формат A6 в пунктах (105 × 148 мм ≈ 298 × 420 pt).
	p.Start(gopdf.Config{PageSize: gopdf.Rect{W: 298, H: 420}})
	if err := p.AddTTFFontData("regular", goregular.TTF); err != nil {
		return nil, err
	}
	if err := p.AddTTFFontData("bold", gobold.TTF); err != nil {
		return nil, err
	}
	p.AddPage()
	botName := d.BotName
	if botName == "" {
		botName = "t105_hakaton_max_bot"
	}

	plate, question, emergency := stickerTexts(d.Category, d.Label)
	if d.EntranceNumber > 0 {
		plate = fmt.Sprintf("%s  |  подъезд %d", plate, d.EntranceNumber)
	}

	p.SetInfo(gopdf.PdfInfo{
		Title:        fmt.Sprintf("Наклейка: %s, %s", d.Address, d.Label),
		Subject:      "QR-наклейка объекта общего имущества дома",
		CreationDate: d.GeneratedAt,
	})

	// Эмалевая шапка-табличка (#1B3F94).
	p.SetFillColor(27, 63, 148)
	p.RectFromUpperLeftWithStyle(16, 16, 266, 56, "F")

	p.SetTextColor(255, 255, 255)
	_ = p.SetFont("regular", "", 9)
	p.SetXY(26, 26)
	_ = p.Cell(nil, d.Address)

	_ = p.SetFont("bold", "", 15)
	p.SetXY(26, 44)
	_ = p.Cell(nil, plate)

	// Заголовок и подзаголовок.
	p.SetTextColor(10, 11, 13)
	_ = p.SetFont("bold", "", 15)
	p.SetXY(16, 86)
	_ = p.Cell(nil, question)

	_ = p.SetFont("regular", "", 9)
	p.SetXY(16, 106)
	_ = p.Cell(nil, "Сообщите соседям и в управляющую компанию за минуту.")

	// Векторный QR-код (если передана матрица модулей) и шаги справа.
	rows := parseQRMatrix(d.QRMatrix)
	textX := 16.0
	if len(rows) > 0 {
		drawQR(p, rows, 16, 126, 108)
		textX = 134.0
	}

	steps := []string{
		"1. Наведите камеру телефона на QR-код.",
		"2. В MAX откроется заявка: дом и объект уже выбраны.",
		"3. Соседи присоединятся, УК увидит число обращений.",
	}
	y := 132.0
	_ = p.SetFont("regular", "", 8.5)
	for _, s := range steps {
		lines, _ := p.SplitTextWithWordWrap(s, 282-textX)
		for _, l := range lines {
			p.SetXY(textX, y)
			_ = p.Cell(nil, l)
			y += 12
		}
		y += 6
	}

	// Ссылка для прямого перехода.
	linkY := 252.0
	p.SetTextColor(80, 86, 98)
	_ = p.SetFont("regular", "", 8)
	p.SetXY(16, linkY)
	_ = p.Cell(nil, fmt.Sprintf("Ссылка: max.ru/%s?startapp=o_%s", botName, d.QRCode))

	// Блок аварийно-диспетчерской службы.
	p.SetFillColor(242, 244, 248)
	p.RectFromUpperLeftWithStyle(16, 276, 266, 58, "F")

	p.SetTextColor(10, 11, 13)
	_ = p.SetFont("bold", "", 9.5)
	p.SetXY(26, 288)
	_ = p.Cell(nil, emergency)

	_ = p.SetFont("regular", "", 8.5)
	p.SetXY(26, 303)
	_ = p.Cell(nil, "Звоните в диспетчерскую, не пишите заявку:")

	phone := d.PhoneDispatcher
	if phone == "" {
		phone = "+7 495 539-53-53"
	}
	_ = p.SetFont("bold", "", 11)
	p.SetXY(26, 317)
	_ = p.Cell(nil, phone+"  ("+d.Organization+")")

	// Подвал наклейки.
	p.SetTextColor(110, 116, 128)
	_ = p.SetFont("regular", "", 8)
	p.SetXY(16, 392)
	_ = p.Cell(nil, fmt.Sprintf("max.ru/%s  •  наклейка формата A6", botName))

	return p.GetBytesPdf(), nil
}

func stickerTexts(category, label string) (plate, question, emergency string) {
	switch category {
	case "lift":
		return "Лифт", "Лифт не работает?", "Застряли в лифте?"
	case "lighting":
		return "Свет", "Не горит свет?", "Авария или короткое замыкание?"
	case "leak":
		return "Кровля", "Протекает крыша?", "Авария или затопление?"
	case "garbage":
		return "Мусоропровод", "Засор мусоропровода?", "Экстренная ситуация?"
	default:
		if label == "" {
			label = "Объект дома"
		}
		return label, "Что-то сломалось?", "Авария или затопление?"
	}
}

func parseQRMatrix(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 15 || len(parts) > 80 {
		return nil
	}
	size := len(parts)
	for _, r := range parts {
		if len(r) != size {
			return nil
		}
		for i := range len(r) {
			if r[i] != '0' && r[i] != '1' {
				return nil
			}
		}
	}
	return parts
}

func drawQR(p *gopdf.GoPdf, rows []string, x, y, boxSize float64) {
	n := float64(len(rows))
	cell := boxSize / n
	p.SetFillColor(255, 255, 255)
	p.RectFromUpperLeftWithStyle(x, y, boxSize, boxSize, "F")
	p.SetFillColor(10, 11, 13)
	for rIdx, row := range rows {
		for cIdx := range len(row) {
			if row[cIdx] == '1' {
				p.RectFromUpperLeftWithStyle(x+float64(cIdx)*cell, y+float64(rIdx)*cell, cell+0.15, cell+0.15, "F")
			}
		}
	}
}
