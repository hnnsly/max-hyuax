package bot

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"dommax/internal/app/cards"
	"dommax/internal/domain/issue"
	"dommax/internal/storage/maxapi"
)

var moscow = time.FixedZone("MSK", 3*60*60)

var months = [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}

// dayMonth — «19 сентября» по Москве: сроки заявок считаются по московскому времени.
func dayMonth(t time.Time) string {
	t = t.In(moscow)
	return fmt.Sprintf("%d %s", t.Day(), months[t.Month()-1])
}

// plain убирает из текста жителя символы разметки markdown, чтобы он не ломал карточку.
var plain = strings.NewReplacer("*", "", "_", " ", "~", "", "`", "", "++", "+", "\n#", "\n", "\n>", "\n").Replace

var statusWord = map[issue.Status]string{
	issue.StatusSent:       "отправлена",
	issue.StatusAccepted:   "принята",
	issue.StatusInProgress: "в работе",
	issue.StatusDone:       "выполнена",
	issue.StatusRejected:   "отклонена",
}

func capitalizeRU(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// RenderCard собирает живую карточку заявки по DESIGN-SYSTEM §5 (без эмодзи и тире).
func RenderCard(c cards.Card, botName string) maxapi.NewMessage {
	var b strings.Builder
	fmt.Fprintf(&b, "**Заявка № %d**\n%s\n", c.Number, plain(c.Title))
	if c.Place != "" {
		b.WriteString(capitalizeRU(plain(c.Place)) + "\n")
	}
	b.WriteString("\n")

	if c.Status.Closed() {
		fmt.Fprintf(&b, "Статус: %s %s\n", statusWord[c.Status], dayMonth(c.StatusAt))
	} else {
		b.WriteString("Статус: " + statusWord[c.Status])
		if c.Responsible != "" {
			b.WriteString(", отвечает " + c.Responsible)
		}
		b.WriteString("\n")
		if c.Overdue {
			fmt.Fprintf(&b, "Срок: истёк %s\n", dayMonth(c.Deadline))
		} else {
			fmt.Fprintf(&b, "Срок: до %s\n", dayMonth(c.Deadline))
		}
	}
	fmt.Fprintf(&b, "Сообщили соседи: %d\n", c.Participants)
	if c.Comment != "" {
		fmt.Fprintf(&b, "\nКомментарий УК: %s\n", plain(c.Comment))
	}
	return maxapi.NewMessage{
		Text:   strings.TrimRight(b.String(), "\n"),
		Format: "markdown",
		Attachments: []maxapi.Attachment{maxapi.Keyboard([]maxapi.Button{
			maxapi.OpenAppButton("Открыть заявку", botName, "i_"+c.IssueID),
			maxapi.LinkButton("Поделиться", shareURL(c, botName)),
		})},
	}
}

// shareURL — ссылка на шеринг в MAX: текст без имён и квартир плюс ссылка на заявку.
func shareURL(c cards.Card, botName string) string {
	text := fmt.Sprintf("Заявка № %d: %s. %s. Если у вас то же самое, присоединяйтесь: https://max.ru/%s?startapp=i_%s",
		c.Number, plain(c.Title), c.Address, botName, c.IssueID)
	return "https://max.ru/:share?text=" + url.QueryEscape(text)
}

// renderFinal — отдельное сообщение участникам, когда заявка закрыта.
func renderFinal(c cards.Card, botName string) maxapi.NewMessage {
	var b strings.Builder
	if c.Status == issue.StatusRejected {
		fmt.Fprintf(&b, "**Заявка № %d отклонена.** %s\n", c.Number, plain(c.Title))
		if c.Comment != "" {
			fmt.Fprintf(&b, "Причина: %s\n", plain(c.Comment))
		}
	} else {
		fmt.Fprintf(&b, "**Заявка № %d выполнена.** %s\n", c.Number, plain(c.Title))
		if c.Comment != "" {
			fmt.Fprintf(&b, "Комментарий УК: %s\n", plain(c.Comment))
		}
		b.WriteString("Если проблема повторится, сообщите о ней заново.")
	}
	return maxapi.NewMessage{
		Text:   strings.TrimRight(b.String(), "\n"),
		Format: "markdown",
		Attachments: []maxapi.Attachment{maxapi.Keyboard([]maxapi.Button{
			maxapi.OpenAppButton("Открыть заявку", botName, "i_"+c.IssueID),
		})},
	}
}

// CardAPI — часть Bot API для карточек.
type CardAPI interface {
	Send(ctx context.Context, to maxapi.Target, m maxapi.NewMessage) (maxapi.Message, error)
	Edit(ctx context.Context, mid string, m maxapi.NewMessage) error
}

// CardSender реализует cards.Messenger поверх Bot API.
type CardSender struct {
	api     CardAPI
	botName string
}

func NewCardSender(api CardAPI, botName string) *CardSender {
	return &CardSender{api: api, botName: botName}
}

func (s *CardSender) UpsertCard(ctx context.Context, maxUserID int64, mid string, c cards.Card) (string, error) {
	m := RenderCard(c, s.botName)
	if mid != "" {
		return mid, s.api.Edit(ctx, mid, m)
	}
	msg, err := s.api.Send(ctx, maxapi.ToUser(maxUserID), m)
	return msg.Body.MID, err
}

func (s *CardSender) SendFinal(ctx context.Context, maxUserID int64, c cards.Card) error {
	_, err := s.api.Send(ctx, maxapi.ToUser(maxUserID), renderFinal(c, s.botName))
	return err
}
