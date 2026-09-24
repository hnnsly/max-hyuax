// Пакет pdf отрисовывает документы для скачивания: черновик обращения в жилинспекцию.
// Шрифт — Go Regular и Go Bold из golang.org/x/image (BSD, есть кириллица): встраивается в бинарник.
package pdf

import (
	"fmt"
	"strings"
	"time"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"

	"dommax/internal/app/appeal"
	"dommax/internal/domain/issue"
)

var moscow = time.FixedZone("MSK", 3*60*60)

// Поля листа A4 в пунктах.
const (
	left   = 56.0
	right  = 539.0
	top    = 56.0
	bottom = 786.0
)

var statusText = map[issue.Status]string{
	issue.StatusSent:       "Заявка отправлена в УК",
	issue.StatusAccepted:   "УК приняла заявку",
	issue.StatusInProgress: "УК взяла заявку в работу",
	issue.StatusDone:       "УК отметила заявку выполненной",
	issue.StatusRejected:   "УК отклонила заявку",
}

// Appeal собирает черновик обращения в Мосжилинспекцию по данным заявки.
func Appeal(d appeal.Document) ([]byte, error) {
	w, err := newWriter()
	if err != nil {
		return nil, err
	}
	w.pdf.SetInfo(gopdf.PdfInfo{
		Title:        fmt.Sprintf("Обращение по заявке № %d", d.Number),
		Subject:      "Черновик обращения в Мосжилинспекцию",
		CreationDate: d.GeneratedAt,
	})

	// Шапка: адресат и строки, которые житель заполняет сам (ПДн в файл не пишем).
	head := 300.0
	w.text("В Государственную жилищную инспекцию города Москвы (Мосжилинспекция)", "bold", 11, head, 4)
	w.text("от ____________________________________", "regular", 11, head, 0)
	w.text("(фамилия, имя, отчество)", "regular", 8, head, 4)
	w.text("адрес: "+d.Address+", кв. ______", "regular", 11, head, 0)
	w.text("телефон: ______________________________", "regular", 11, head, 18)

	w.text("Обращение", "bold", 15, left, 2)
	w.text("о неисполнении управляющей организацией обязанностей по содержанию общего имущества многоквартирного дома", "regular", 11, left, 14)

	about := fmt.Sprintf("%s жители дома по адресу %s сообщили в управляющую организацию %s о проблеме: %s.",
		date(d.CreatedAt), d.Address, d.Organization, strings.TrimSuffix(d.Title, "."))
	if d.Category != "" {
		about += " Категория: " + d.Category + "."
	}
	if d.Place != "" {
		about += " Место: " + d.Place + "."
	}
	w.text(about, "regular", 11, left, 6)
	if d.Description != "" {
		w.text("Описание жителей: «"+d.Description+"»", "regular", 11, left, 6)
	}
	due := fmt.Sprintf("Ответ должен был поступить не позднее %s, срок истёк.", date(d.Deadline))
	if d.Basis != "" {
		due += " Основание срока: " + d.Basis + "."
	}
	w.text(due, "regular", 11, left, 6)
	w.text(fmt.Sprintf("Число жителей, сообщивших о проблеме: %d.", d.Participants), "regular", 11, left, 12)

	if lines := timelineLines(d.Events); len(lines) > 0 {
		w.text(fmt.Sprintf("Хронология заявки № %d", d.Number), "bold", 11, left, 4)
		for _, l := range lines {
			w.text(l, "regular", 10, left, 2)
		}
		w.y += 10
	}

	w.text("Прошу провести проверку, обязать управляющую организацию устранить нарушение и дать ответ жителям.", "regular", 11, left, 24)
	w.text("Дата: «____» ______________ 20___ г.          Подпись: ____________________", "regular", 11, left, 28)
	w.text(fmt.Sprintf("Черновик подготовлен %s по данным заявки № %d в сервисе «Заявки по дому в MAX». "+
		"Проверьте, дополните и подпишите перед отправкой. Подать обращение можно на mos.ru или лично в Мосжилинспекции.",
		date(d.GeneratedAt), d.Number), "regular", 8, left, 0)
	if w.err != nil {
		return nil, w.err
	}
	return w.pdf.GetBytesPdf(), nil
}

// timelineLines — хронология без имён: присоединения подряд склеиваются в одну строку.
func timelineLines(events []issue.Event) []string {
	var out []string
	joined, lastJoin := 0, time.Time{}
	flush := func() {
		if joined == 0 {
			return
		}
		verb := "Присоединились"
		if joined%10 == 1 && joined%100 != 11 {
			verb = "Присоединился"
		}
		out = append(out, fmt.Sprintf("%s  %s ещё %d %s", stamp(lastJoin), verb, joined, plural(joined, "сосед", "соседа", "соседей")))
		joined = 0
	}
	for _, e := range events {
		if e.Kind == issue.EventJoined {
			joined, lastJoin = joined+1, e.At
			continue
		}
		flush()
		text := statusText[e.Status]
		switch e.Kind {
		case issue.EventCreated:
			text = "Житель сообщил о проблеме"
		case issue.EventOverdue:
			text = "Срок ответа истёк"
		}
		if e.Comment != "" {
			text += ". Комментарий УК: «" + e.Comment + "»"
		}
		out = append(out, stamp(e.At)+"  "+text)
	}
	flush()
	return out
}

func plural(n int, one, few, many string) string {
	switch n10, n100 := n%10, n%100; {
	case n10 == 1 && n100 != 11:
		return one
	case n10 >= 2 && n10 <= 4 && (n100 < 12 || n100 > 14):
		return few
	}
	return many
}

func date(t time.Time) string  { return t.In(moscow).Format("02.01.2006") }
func stamp(t time.Time) string { return t.In(moscow).Format("02.01.2006 15:04") }

// writer пишет абзацы с переносом слов и новыми страницами; первая ошибка запоминается.
type writer struct {
	pdf *gopdf.GoPdf
	y   float64
	err error
}

func newWriter() (*writer, error) {
	p := &gopdf.GoPdf{}
	p.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	if err := p.AddTTFFontData("regular", goregular.TTF); err != nil {
		return nil, err
	}
	if err := p.AddTTFFontData("bold", gobold.TTF); err != nil {
		return nil, err
	}
	p.AddPage()
	return &writer{pdf: p, y: top}, nil
}

func (w *writer) text(s, font string, size, x, gapAfter float64) {
	if w.err != nil {
		return
	}
	if w.err = w.pdf.SetFont(font, "", size); w.err != nil {
		return
	}
	lines, err := w.pdf.SplitTextWithWordWrap(s, right-x)
	if err != nil {
		w.err = err
		return
	}
	lineHeight := size * 1.35
	for _, l := range lines {
		if w.y+lineHeight > bottom {
			w.pdf.AddPage()
			w.y = top
		}
		w.pdf.SetXY(x, w.y)
		if w.err = w.pdf.Cell(nil, l); w.err != nil {
			return
		}
		w.y += lineHeight
	}
	w.y += gapAfter
}
