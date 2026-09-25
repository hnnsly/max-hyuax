package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/signintech/gopdf"

	"dommax/internal/domain/issue"
)

// HouseReportDocument — данные для генерации сводного отчёта по дому.
type HouseReportDocument struct {
	Address        string
	District       string
	Organization   string
	Dispatcher     string
	GeneratedAt    time.Time
	TotalIssues    int
	OpenIssues     int
	DoneIssues     int
	OverdueIssues  int
	TotalReporters int
	Issues         []HouseReportIssue
}

type HouseReportIssue struct {
	Number       int64
	Title        string
	Category     string
	Status       issue.Status
	CreatedAt    time.Time
	Deadline     time.Time
	Participants int
	Comment      string
}

// HouseReport формирует сводный отчёт о состоянии общего имущества дома в формате PDF.
func HouseReport(d HouseReportDocument) ([]byte, error) {
	w, err := newWriter()
	if err != nil {
		return nil, err
	}
	w.pdf.SetInfo(gopdf.PdfInfo{
		Title:        fmt.Sprintf("Сводный отчёт по дому %s", d.Address),
		Subject:      "Состояние общего имущества и реестр заявок",
		CreationDate: d.GeneratedAt,
	})

	// Шапка документа
	w.text("СВОДНЫЙ ОТЧЁТ ПО ДОМУ", "bold", 15, left, 2)
	w.text("Состояние общего имущества, заявки жителей и соблюдение сроков управляющей организацией", "regular", 10, left, 12)

	// Основные реквизиты дома
	w.text("Адрес: "+d.Address, "bold", 11, left, 3)
	if d.District != "" {
		w.text("Район / округ: "+d.District, "regular", 11, left, 3)
	}
	w.text("Управляющая организация: "+d.Organization, "regular", 11, left, 3)
	if d.Dispatcher != "" {
		w.text("Диспетчерская служба: "+d.Dispatcher, "regular", 11, left, 3)
	}
	w.text("Дата формирования отчёта: "+stamp(d.GeneratedAt), "regular", 9, left, 14)

	// Статистический блок
	w.text("Статистика заявок", "bold", 12, left, 4)
	stats := fmt.Sprintf("Всего заявок: %d  ·  В работе: %d  ·  Выполнено: %d  ·  С нарушением срока: %d  ·  Всего обращений жителей: %d",
		d.TotalIssues, d.OpenIssues, d.DoneIssues, d.OverdueIssues, d.TotalReporters)
	w.text(stats, "regular", 10, left, 14)

	// Реестр проблем
	w.text("Реестр выявленных неисправностей", "bold", 12, left, 6)
	if len(d.Issues) == 0 {
		w.text("Зарегистрированных заявок нет.", "regular", 10, left, 10)
	} else {
		for i, is := range d.Issues {
			header := fmt.Sprintf("%d. Заявка № %d: %s", i+1, is.Number, strings.TrimSuffix(is.Title, "."))
			w.text(header, "bold", 10, left, 2)

			statusLabel := statusText[is.Status]
			if statusLabel == "" {
				statusLabel = string(is.Status)
			}
			details := fmt.Sprintf("   Категория: %s  ·  Подана: %s  ·  Срок ответа: %s  ·  Сообщили жителей: %d",
				is.Category, date(is.CreatedAt), date(is.Deadline), is.Participants)
			w.text(details, "regular", 9, left, 2)

			stLine := fmt.Sprintf("   Текущий статус: %s", statusLabel)
			if is.Comment != "" {
				stLine += fmt.Sprintf(" (комментарий УК: «%s»)", strings.TrimSpace(is.Comment))
			}
			w.text(stLine, "regular", 9, left, 6)
		}
	}

	w.y += 10
	w.text("Документ сформирован в сервисе «Адресная табличка» на базе платформы MAX.", "regular", 8, left, 2)
	w.text("Используется для информирования собственников помещений и отчёта совета многоквартирного дома.", "regular", 8, left, 0)

	if w.err != nil {
		return nil, w.err
	}
	var buf bytes.Buffer
	if _, err := w.pdf.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
