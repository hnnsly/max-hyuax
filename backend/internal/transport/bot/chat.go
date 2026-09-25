package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
	"dommax/internal/storage/maxapi"
)

// Меню и заявки прямо в чате: житель видит карточку, присоединяется и проверяет ремонт
// без мини-приложения. Приложение остаётся для формы с фото и кабинетов.

const (
	cbMenu   = "m" // m:<пункт> — пункт главного меню
	cbIssue  = "i" // i:<issue_id> — карточка заявки в чате
	cbReopen = "r" // r:<issue_id> — «не починили», дальше бот ждёт комментарий

	menuMine  = "mine"
	menuNow   = "now" // открытые заявки дома
	menuHouse = "house"

	// Действия, которые ждут следующего сообщения жителя (app.BotPending).
	pendReopen = "reopen"

	// pendingTTL — сколько бот ждёт текст после своего вопроса; дальше сообщение снова считается проблемой.
	pendingTTL = 10 * time.Minute
	// listLimit — сколько заявок показывать кнопками: больше в одном сообщении читать неудобно.
	listLimit = 8
)

func menuKeyboard(botName string) maxapi.Attachment {
	return maxapi.Keyboard(
		[]maxapi.Button{maxapi.CallbackButton("Сообщить о проблеме", PayloadReport)},
		[]maxapi.Button{maxapi.CallbackButton("Мои заявки", pack(cbMenu, menuMine)), maxapi.CallbackButton("Сейчас в доме", pack(cbMenu, menuNow))},
		[]maxapi.Button{maxapi.CallbackButton("Мой дом", pack(cbMenu, menuHouse))},
		[]maxapi.Button{maxapi.OpenAppButton("Открыть приложение", botName, "")},
	)
}

// greetKeyboard — меню под приветствием; payload диплинка передаётся в мини-приложение.
func greetKeyboard(botName, startPayload string) maxapi.Attachment {
	if startPayload == "" {
		return menuKeyboard(botName)
	}
	return maxapi.Keyboard(
		[]maxapi.Button{maxapi.OpenAppButton("Открыть приложение", botName, startPayload)},
		[]maxapi.Button{maxapi.CallbackButton("Сообщить о проблеме", PayloadReport)},
	)
}

func (h *Handler) menu(ctx context.Context, to maxapi.Target) error {
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{Text: "Что сделать?", Attachments: []maxapi.Attachment{menuKeyboard(h.botName)}})
	return err
}

// menuItem выполняет пункт меню отдельным сообщением: само меню остаётся в чате.
func (h *Handler) menuItem(ctx context.Context, to maxapi.Target, from maxapi.User, item string) (maxapi.CallbackAnswer, error) {
	var err error
	switch item {
	case menuMine:
		err = h.myIssues(ctx, to, from)
	case menuNow:
		err = h.houseNow(ctx, to, from)
	case menuHouse:
		err = h.myHouse(ctx, to, from)
	default:
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела. Откройте меню: /menu"}, nil
	}
	return maxapi.CallbackAnswer{Notification: "Готово"}, err
}

// issueList — заявки кнопками; нажатие открывает карточку прямо в чате.
func (h *Handler) issueList(ctx context.Context, to maxapi.Target, title, empty string, list []*issue.Issue) error {
	if len(list) == 0 {
		return h.send(ctx, to, empty)
	}
	var rows [][]maxapi.Button
	for _, is := range list[:min(listLimit, len(list))] {
		label := truncate(fmt.Sprintf("№ %d, %s", is.Number(), is.Title()), 60)
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton(label, pack(cbIssue, is.ID()))})
	}
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{Text: title, Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}})
	return err
}

// houseNow — открытые заявки дома жителя: можно присоединиться, не создавая дубль.
func (h *Handler) houseNow(ctx context.Context, to maxapi.Target, from maxapi.User) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	if u.HouseID == "" {
		return h.askHouse(ctx, to)
	}
	list, err := h.svc.Issues.ListByHouse(ctx, u.HouseID)
	if err != nil {
		return err
	}
	var open []*issue.Issue
	for _, is := range list {
		if !is.Status().Closed() {
			open = append(open, is)
		}
	}
	return h.issueList(ctx, to, "Сейчас в доме открыты заявки. Если у вас то же самое, откройте заявку и присоединитесь:",
		"В доме нет открытых заявок. Если что-то сломалось, напишите мне одним сообщением.", open)
}

// showIssue присылает карточку заявки с кнопками по её состоянию и роли жителя.
func (h *Handler) showIssue(ctx context.Context, to maxapi.Target, u user.User, issueID string) error {
	is, err := h.svc.Issues.Get(ctx, issueID)
	if err != nil {
		return err
	}
	c, err := h.svc.Cards.Card(ctx, issueID)
	if err != nil {
		return err
	}
	m := RenderCard(c, h.botName)
	if events, err := h.svc.Issues.Timeline(ctx, issueID); err == nil && len(events) > 0 {
		var b strings.Builder
		b.WriteString(m.Text + "\n\n**Хронология**")
		for _, e := range events[max(0, len(events)-3):] {
			fmt.Fprintf(&b, "\n%s: %s", dayMonth(e.At), eventLine(e))
		}
		m.Text = b.String()
	}

	var rows [][]maxapi.Button
	switch {
	case repairAsk(is, u, h.svc.Now()):
		rows = append(rows, []maxapi.Button{
			maxapi.CallbackButton("Починили", ConfirmPayload(is.ID())),
			maxapi.CallbackButton("Не починили", pack(cbReopen, is.ID())),
		})
	case !is.Status().Closed() && !is.HasParticipant(u.ID) && u.CanTakePart() && u.HouseID == is.HouseID():
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Это и у меня", pack(cbJoin, is.ID()))})
	}
	rows = append(rows, []maxapi.Button{
		maxapi.OpenAppButton("Открыть в приложении", h.botName, "i_"+is.ID()),
		maxapi.LinkButton("Поделиться", shareURL(c, h.botName)),
	})
	m.Attachments = []maxapi.Attachment{maxapi.Keyboard(rows...)}
	_, err = h.max.Send(ctx, to, m)
	return err
}

// repairAsk — житель может ответить «починили» или «не починили»: он участник, заявка выполнена,
// 7 дней ещё не прошли и ответа на эту отметку «выполнено» ещё нет.
func repairAsk(is *issue.Issue, u user.User, now time.Time) bool {
	if !is.HasParticipant(u.ID) || is.Status() != issue.StatusDone || !now.Before(is.AnswerUntil()) {
		return false
	}
	_, answered := is.AnswerOf(u.ID)
	return !answered
}

// eventLine — строка хронологии без имён: кто именно действовал, не раскрывается.
func eventLine(e issue.Event) string {
	switch e.Kind {
	case issue.EventCreated:
		return "заявка подана"
	case issue.EventJoined:
		return "присоединился сосед"
	case issue.EventStatusChanged:
		line := "статус «" + statusWord[e.Status] + "»"
		if e.Comment != "" {
			line += ": " + plain(e.Comment)
		}
		return line
	case issue.EventOverdue:
		return "истёк срок ответа"
	case issue.EventConfirmed:
		return "сосед подтвердил, что починили"
	case issue.EventReopened:
		return "житель вернул в работу: не починили"
	}
	return "изменение"
}

// askReopenComment запоминает, что следующий текст жителя — комментарий к «не починили».
func (h *Handler) askReopenComment(ctx context.Context, u user.User, to maxapi.Target, issueID string) (maxapi.CallbackAnswer, error) {
	err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendReopen, Ref: issueID, ExpiresAt: h.svc.Now().Add(pendingTTL)})
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	err = h.send(ctx, to, "Напишите одним сообщением, что осталось не так. Заявка вернётся в работу с новым сроком, соседи получат уведомление.")
	return maxapi.CallbackAnswer{Notification: "Жду комментарий"}, err
}

// onPending отдаёт текст действию, которое его ждёт. handled = false — ждать было нечего.
func (h *Handler) onPending(ctx context.Context, to maxapi.Target, u user.User, txt string) (handled bool, err error) {
	p, ok, err := h.svc.Pending.Take(ctx, u.ID, h.svc.Now())
	if err != nil || !ok {
		return false, err
	}
	switch p.Action {
	case pendReopen:
		is, err := h.svc.Issues.Reopen(ctx, u, p.Ref, txt)
		switch {
		case errors.Is(err, issue.ErrWindowClosed):
			return true, h.send(ctx, to, "Ответить можно в течение 7 дней после отметки о выполнении. Этот срок прошёл.")
		case errors.Is(err, issue.ErrAlreadyAnswered):
			return true, h.send(ctx, to, "Вы уже ответили по этому ремонту.")
		case err != nil:
			return true, err
		}
		return true, h.send(ctx, to, fmt.Sprintf("Заявка № %d снова в работе. Новый срок ответа до %s. Соседи получат уведомление.",
			is.Number(), dayMonth(is.Deadline())))
	}
	return false, nil
}
