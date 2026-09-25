package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
	"dommax/internal/storage/maxapi"
)

// Заявки прямо в чате: житель видит карточку, присоединяется и проверяет ремонт
// без мини-приложения. Приложение остаётся для формы с фото и кабинетов. Экраны — в screens.go.

const (
	cbMenu   = "m" // m:<экран> — экран новым сообщением: кнопка «Меню» в уведомлениях и живой карточке
	cbIssue  = "i" // i:<issue_id> — карточка заявки новым сообщением («Подробнее» в уведомлении)
	cbReopen = "r" // r:<issue_id> — «не починили», дальше бот ждёт комментарий
	cbPlace  = "p" // p:<object_id> или p:- — место проблемы; описание ждёт в app.BotPending

	// Действия, которые ждут следующего сообщения жителя (app.BotPending).
	pendReopen = "reopen"
	pendPlace  = "place" // Ref — категория, Text — описание проблемы

	// pendingTTL — сколько бот ждёт текст после своего вопроса; дальше сообщение снова считается проблемой.
	pendingTTL = 10 * time.Minute
	// listLimit — сколько заявок показывать кнопками: больше в одном сообщении читать неудобно.
	listLimit = 8
)

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
func (h *Handler) askReopenComment(ctx context.Context, u user.User, issueID string) (maxapi.CallbackAnswer, error) {
	err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendReopen, Ref: issueID, ExpiresAt: h.svc.Now().Add(pendingTTL)})
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	m := screenMsg("Напишите одним сообщением, что осталось не так. Заявка вернётся в работу с новым сроком, соседи получат уведомление.",
		navRow(scrIssue, issueID))
	return maxapi.CallbackAnswer{Message: &m}, nil
}

// dropPending забывает, чего бот ждал: житель ушёл на другой экран, и следующий текст снова считается проблемой.
func (h *Handler) dropPending(ctx context.Context, u user.User) {
	if _, _, err := h.svc.Pending.Take(ctx, u.ID, h.svc.Now()); err != nil {
		h.log.WarnContext(ctx, "bot pending drop failed", "err", err)
	}
}

// onPending отдаёт текст действию, которое его ждёт. handled = false — ждать было нечего.
func (h *Handler) onPending(ctx context.Context, to maxapi.Target, u user.User, txt string) (handled bool, err error) {
	p, ok, err := h.svc.Pending.Take(ctx, u.ID, h.svc.Now())
	if err != nil || !ok {
		return false, err
	}
	switch p.Action {
	case pendPropose, pendDecline, pendNewPoll:
		return true, h.onCouncilPending(ctx, to, u, p, txt)
	case pendDescribe:
		m, err := h.reportMessage(ctx, u, p.Ref, p.Text, txt)
		if err != nil {
			return true, err
		}
		_, err = h.max.Send(ctx, to, m)
		return true, err
	case pendStatus:
		return true, h.statusComment(ctx, to, u, p, txt)
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

// askPlace спрашивает подъезд, если у дома несколько объектов этой категории (лифт, свет).
// Описание не помещается в данные кнопки, поэтому ждёт в app.BotPending.
func (h *Handler) askPlace(ctx context.Context, u user.User, category, desc string) (maxapi.CallbackAnswer, bool, error) {
	d, err := h.svc.Houses.Get(ctx, u.HouseID)
	if err != nil {
		return maxapi.CallbackAnswer{}, false, err
	}
	var rows [][]maxapi.Button
	for _, o := range d.Objects {
		if o.Category == category && o.EntranceID != "" {
			rows = append(rows, []maxapi.Button{maxapi.CallbackButton(truncate(capitalizeRU(o.Label), 40), pack(cbPlace, o.ID))})
		}
	}
	if len(rows) < 2 {
		return maxapi.CallbackAnswer{}, false, nil
	}
	err = h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendPlace, Ref: category, Text: desc, ExpiresAt: h.svc.Now().Add(pendingTTL)})
	if err != nil {
		return maxapi.CallbackAnswer{}, false, err
	}
	rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Не знаю или во всём доме", pack(cbPlace, "-"))}, menuRow())
	m := maxapi.NewMessage{Text: "Где именно? Так УК быстрее найдёт место, а соседи из того же подъезда увидят заявку.", Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}}
	return maxapi.CallbackAnswer{Message: &m}, true, nil
}

// placeChosen отправляет заявку с выбранным местом.
func (h *Handler) placeChosen(ctx context.Context, u user.User, objectID string) (maxapi.CallbackAnswer, error) {
	p, ok, err := h.svc.Pending.Take(ctx, u.ID, h.svc.Now())
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	if !ok || p.Action != pendPlace {
		return replace("Этот вопрос устарел. Напишите о проблеме ещё раз."), nil
	}
	if objectID == "-" {
		objectID = ""
	}
	// Вдруг по этому месту заявка уже есть: лучше присоединиться, чем плодить дубль.
	if similar, err := h.svc.Issues.FindSimilar(ctx, u.HouseID, p.Ref, objectID); err == nil && len(similar) > 0 && objectID != "" {
		return h.join(ctx, u, similar[0].ID())
	}
	return h.report(ctx, u, p.Ref, objectID, p.Text)
}

// onContact сохраняет телефон для мастера из кнопки «Оставить телефон для мастера».
func (h *Handler) onContact(ctx context.Context, to maxapi.Target, from maxapi.User, a maxapi.IncomingAttachment) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	vcf, hash, ok := a.Contact()
	if ok {
		_, err = h.svc.Auth.SharePhoneFromBot(ctx, u, vcf, hash)
	}
	if !ok || errors.Is(err, app.ErrInvalidInput) {
		return h.sendKeyboard(ctx, to, "Не получилось подтвердить номер. Нажмите кнопку ниже: пересланный контакт или номер текстом не подходят.",
			[]maxapi.Button{maxapi.ContactButton("Оставить телефон для мастера")})
	}
	if err != nil {
		return err
	}
	return h.send(ctx, to, "Телефон сохранён. Его увидит только управляющая компания по вашим открытым заявкам, соседи номер не видят. Убрать номер можно в приложении.")
}
