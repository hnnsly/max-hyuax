package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dommax/internal/app"
	appcouncil "dommax/internal/app/council"
	"dommax/internal/domain/council"
	"dommax/internal/domain/user"
	"dommax/internal/storage/maxapi"
)

// Совет дома в чате (ADR-017): опросы с голосованием кнопками, предложение председателю,
// ответы председателя. Опрос из предложения открывается в приложении: там правится вопрос и варианты.

const (
	cbVote    = "v" // v:<poll_id>:<номер варианта>
	cbCouncil = "s" // s:<пункт> — раздел «Совет дома»
	cbAccept  = "a" // a:<proposal_id> — председатель берёт в работу
	cbDecline = "d" // d:<proposal_id> — председатель отклоняет, ответ ждёт следующим сообщением

	menuCouncil    = "council"
	councilPropose = "propose"
	councilMine    = "mine"
	councilFolder  = "folder"
	councilNewPoll = "new_poll"

	pendPropose = "propose"
	pendDecline = "decline" // Ref — id предложения
	pendNewPoll = "new_poll"

	// pollsInChat — сколько открытых опросов присылать сразу, остальные в приложении.
	pollsInChat = 3
)

var proposalStatusText = map[council.Status]string{
	council.StatusNew:      "ждёт ответа председателя",
	council.StatusAccepted: "председатель взял в работу",
	council.StatusDeclined: "председатель отклонил",
}

// councilMenu присылает открытые опросы дома и кнопки совета.
func (h *Handler) councilMenu(ctx context.Context, to maxapi.Target, from maxapi.User) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	if u.HouseID == "" {
		return h.askHouse(ctx, to)
	}
	polls, err := h.svc.Council.Polls(ctx, u)
	if err != nil {
		return err
	}
	open := 0
	for _, p := range polls {
		if p.Open && open < pollsInChat {
			open++
			if _, err := h.max.Send(ctx, to, pollMessage(p)); err != nil {
				return err
			}
		}
	}
	text := "Совет дома: предложения председателю и опросы соседей."
	if open == 0 {
		text += " Открытых опросов сейчас нет."
	}
	rows := [][]maxapi.Button{
		{maxapi.CallbackButton("Предложить совету", pack(cbCouncil, councilPropose))},
		{maxapi.CallbackButton("Мои предложения", pack(cbCouncil, councilMine))},
	}
	if u.IsChairmanOf(u.HouseID) {
		rows = append(rows, []maxapi.Button{
			maxapi.CallbackButton("Папка предложений", pack(cbCouncil, councilFolder)),
			maxapi.CallbackButton("Создать опрос", pack(cbCouncil, councilNewPoll)),
		})
	}
	_, err = h.max.Send(ctx, to, maxapi.NewMessage{Text: text, Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}})
	return err
}

// pollMessage — опрос: до голоса варианты кнопками, после голоса или окончания итоги.
func pollMessage(v appcouncil.PollView) maxapi.NewMessage {
	var b strings.Builder
	fmt.Fprintf(&b, "**Опрос дома.** %s\n", plain(v.Question))
	if v.Open {
		fmt.Fprintf(&b, "Ответить можно до %s. ", dayMonth(v.ClosesAt))
	} else {
		b.WriteString("Опрос закончился. ")
	}
	b.WriteString("Без юридической силы: это не общее собрание собственников.")
	m := maxapi.NewMessage{Format: "markdown"}
	if v.Open && v.Mine < 0 {
		var rows [][]maxapi.Button
		for i, o := range v.Options {
			rows = append(rows, []maxapi.Button{maxapi.CallbackButton(truncate(o, 60), pack(cbVote, v.ID, strconv.Itoa(i)))})
		}
		m.Attachments = []maxapi.Attachment{maxapi.Keyboard(rows...)}
	} else {
		b.WriteString("\n\n" + pollResults(v))
	}
	m.Text = b.String()
	return m
}

// pollResults — итоги в процентах; округление наибольшим остатком, чтобы сумма была ровно 100
// (так же считает мини-приложение, miniapp/src/shared/lib/council.ts).
func pollResults(v appcouncil.PollView) string {
	total := 0
	for _, n := range v.Votes {
		total += n
	}
	share := make([]int, len(v.Votes))
	if total > 0 {
		rest := 100
		frac := make([]float64, len(v.Votes))
		for i, n := range v.Votes {
			exact := float64(n*100) / float64(total)
			share[i] = int(exact)
			frac[i] = exact - float64(share[i])
			rest -= share[i]
		}
		for ; rest > 0; rest-- {
			best := 0
			for i := range frac {
				if frac[i] > frac[best] {
					best = i
				}
			}
			share[best]++
			frac[best] = -1
		}
	}
	var b strings.Builder
	for i, o := range v.Options {
		fmt.Fprintf(&b, "%s: %d%%", plain(o), share[i])
		if i == v.Mine {
			b.WriteString(", ваш голос")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Всего %d %s", total, pluralRU(total, "голос", "голоса", "голосов"))
	return b.String()
}

func pluralRU(n int, one, few, many string) string {
	switch n10, n100 := n%10, n%100; {
	case n10 == 1 && n100 != 11:
		return one
	case n10 >= 2 && n10 <= 4 && (n100 < 12 || n100 > 14):
		return few
	}
	return many
}

func (h *Handler) vote(ctx context.Context, u user.User, payload, rest string) (maxapi.CallbackAnswer, error) {
	pollID, n, _ := strings.Cut(rest, ":")
	option, err := strconv.Atoi(n)
	if err != nil {
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
	}
	v, err := h.svc.Council.Vote(ctx, u, pollID, option)
	switch {
	case errors.Is(err, app.ErrConsentRequired):
		return consentAnswer("Чтобы проголосовать, нужно ваше согласие на обработку персональных данных. Соседи видят только итоги, без имён.", payload), nil
	case errors.Is(err, council.ErrAlreadyVoted):
		return maxapi.CallbackAnswer{Notification: "Вы уже проголосовали в этом опросе."}, nil
	case errors.Is(err, council.ErrPollClosed):
		return maxapi.CallbackAnswer{Notification: "Опрос уже закончился."}, nil
	case errors.Is(err, app.ErrNotFound), errors.Is(err, app.ErrForbidden), errors.Is(err, council.ErrInvalid):
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
	case err != nil:
		return maxapi.CallbackAnswer{}, err
	}
	m := pollMessage(v)
	return maxapi.CallbackAnswer{Message: &m}, nil
}

// consentAnswer — просьба о согласии; после «Согласен» исходная кнопка выполнится сама.
func consentAnswer(text, payload string) maxapi.CallbackAnswer {
	m := maxapi.NewMessage{
		Text:        text + " Данные хранятся в России.",
		Attachments: []maxapi.Attachment{maxapi.Keyboard([]maxapi.Button{maxapi.CallbackButton("Согласен", pack(cbConsent, payload))})},
	}
	return maxapi.CallbackAnswer{Message: &m}
}

func (h *Handler) councilItem(ctx context.Context, cb *maxapi.Callback, u user.User, item string) (maxapi.CallbackAnswer, error) {
	to := maxapi.ToUser(cb.User.UserID)
	switch item {
	case councilPropose:
		if u.HouseID == "" {
			return maxapi.CallbackAnswer{Notification: "Сначала укажите дом"}, h.askHouse(ctx, to)
		}
		if !u.HasConsent(h.svc.ConsentVersion) {
			return consentAnswer("Чтобы написать совету, нужно ваше согласие на обработку персональных данных. Председатель увидит текст без вашего имени.", cb.Payload), nil
		}
		if err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendPropose, ExpiresAt: h.svc.Now().Add(pendingTTL)}); err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		err := h.send(ctx, to, "Напишите предложение одним сообщением: что и где стоит сделать. Председатель совета увидит текст без вашего имени.")
		return maxapi.CallbackAnswer{Notification: "Жду предложение"}, err

	case councilMine:
		list, err := h.svc.Council.MyProposals(ctx, u)
		if err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		if len(list) == 0 {
			return maxapi.CallbackAnswer{Notification: "Предложений пока нет"}, nil
		}
		var b strings.Builder
		b.WriteString("**Мои предложения**")
		for i, p := range list[:min(listLimit, len(list))] {
			fmt.Fprintf(&b, "\n\n%d. %s\n%s", i+1, plain(truncate(p.Text, 200)), capitalizeRU(proposalStatusText[p.Status]))
			if p.Answer != "" {
				fmt.Fprintf(&b, ": %s", plain(p.Answer))
			}
		}
		_, err = h.max.Send(ctx, to, maxapi.NewMessage{Text: b.String(), Format: "markdown"})
		return maxapi.CallbackAnswer{Notification: "Готово"}, err

	case councilFolder:
		list, err := h.svc.Council.Folder(ctx, u)
		if errors.Is(err, app.ErrForbidden) {
			return maxapi.CallbackAnswer{Notification: "Папка доступна председателю совета."}, nil
		}
		if err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		fresh := 0
		for _, p := range list {
			if p.Status == council.StatusNew && fresh < listLimit {
				fresh++
				if _, err := h.max.Send(ctx, to, proposalMessage(p, h.botName)); err != nil {
					return maxapi.CallbackAnswer{}, err
				}
			}
		}
		if fresh == 0 {
			return maxapi.CallbackAnswer{Notification: "Новых предложений нет"}, nil
		}
		return maxapi.CallbackAnswer{Notification: "Новые предложения ниже"}, nil

	case councilNewPoll:
		if !u.IsChairmanOf(u.HouseID) {
			return maxapi.CallbackAnswer{Notification: "Создавать опросы может только председатель совета."}, nil
		}
		if err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendNewPoll, ExpiresAt: h.svc.Now().Add(pendingTTL)}); err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		err := h.send(ctx, to, "Напишите вопрос опроса для жителей одним сообщением (например: «Установить шлагбаум на въезде во двор?»).\n\nОпрос откроется на 7 дней с вариантами «За», «Против», «Воздержался».")
		return maxapi.CallbackAnswer{Notification: "Жду вопрос опроса"}, err
	}
	return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
}

// proposalMessage — новое предложение для председателя, без имени автора.
func proposalMessage(p council.Proposal, botName string) maxapi.NewMessage {
	return maxapi.NewMessage{
		Text:   fmt.Sprintf("**Предложение по дому, %s.**\n%s\n\nИмя автора не показывается.", dayMonth(p.CreatedAt), plain(p.Text)),
		Format: "markdown",
		Attachments: []maxapi.Attachment{maxapi.Keyboard(
			[]maxapi.Button{maxapi.CallbackButton("Взять в работу", pack(cbAccept, p.ID)), maxapi.CallbackButton("Отклонить", pack(cbDecline, p.ID))},
			[]maxapi.Button{maxapi.OpenAppButton("Вынести на опрос в приложении", botName, "")},
		)},
	}
}

// replyAnswer переводит ошибки ответа председателя в понятные уведомления.
func replyAnswer(err error) (maxapi.CallbackAnswer, error) {
	switch {
	case errors.Is(err, council.ErrAlreadyAnswered):
		return maxapi.CallbackAnswer{Notification: "На это предложение уже ответили."}, nil
	case errors.Is(err, app.ErrNotFound):
		return maxapi.CallbackAnswer{Notification: "Предложение не найдено."}, nil
	}
	return maxapi.CallbackAnswer{}, err
}

func (h *Handler) acceptProposal(ctx context.Context, u user.User, id string) (maxapi.CallbackAnswer, error) {
	if _, err := h.svc.Council.Reply(ctx, u, id, council.StatusAccepted, ""); err != nil {
		return replyAnswer(err)
	}
	return replace("Предложение взято в работу, автор получит уведомление. Вынести вопрос на опрос соседей можно в приложении."), nil
}

func (h *Handler) askDeclineAnswer(ctx context.Context, u user.User, id string) (maxapi.CallbackAnswer, error) {
	err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendDecline, Ref: id, ExpiresAt: h.svc.Now().Add(pendingTTL)})
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	err = h.send(ctx, maxapi.ToUser(u.MaxUserID), "Напишите одним сообщением, почему отклоняете. Автор увидит этот ответ.")
	return maxapi.CallbackAnswer{Notification: "Жду ответ автору"}, err
}

// onCouncilPending — текст предложения или ответа председателя, которого ждал бот.
func (h *Handler) onCouncilPending(ctx context.Context, to maxapi.Target, u user.User, p app.BotPending, txt string) error {
	switch p.Action {
	case pendPropose:
		_, err := h.svc.Council.Propose(ctx, u, txt)
		if errors.Is(err, council.ErrInvalid) {
			// Слишком коротко или длинно: ждём исправленный текст ещё раз.
			_ = h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendPropose, ExpiresAt: h.svc.Now().Add(pendingTTL)})
			return h.send(ctx, to, fmt.Sprintf("Предложение должно быть от %d до %d символов. Напишите его ещё раз.", council.MinTextRunes, council.MaxTextRunes))
		}
		if err != nil {
			return err
		}
		return h.send(ctx, to, "Предложение отправлено председателю совета. Ответ придёт сюда, он же появится в «Моих предложениях».")
	case pendDecline:
		if _, err := h.svc.Council.Reply(ctx, u, p.Ref, council.StatusDeclined, txt); err != nil {
			a, err := replyAnswer(err)
			if err != nil {
				return err
			}
			return h.send(ctx, to, a.Notification)
		}
		return h.send(ctx, to, "Предложение отклонено, автор увидит ваш ответ.")
	case pendNewPoll:
		v, err := h.svc.Council.CreatePoll(ctx, u, appcouncil.PollInput{
			Question: txt,
			Options:  []string{"За", "Против", "Воздержался"},
			Days:     7,
		})
		if errors.Is(err, council.ErrInvalid) {
			_ = h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendNewPoll, ExpiresAt: h.svc.Now().Add(pendingTTL)})
			return h.send(ctx, to, "Вопрос должен содержать от 5 до 300 символов. Напишите его ещё раз.")
		}
		if err != nil {
			return err
		}
		_, err = h.max.Send(ctx, to, pollMessage(v))
		return err
	}
	return nil
}

// NotifyCouncil реализует cards.Messenger: председателю о новом предложении, автору об ответе.
func (s *CardSender) NotifyCouncil(ctx context.Context, maxUserID int64, kind app.NotificationKind, p council.Proposal) error {
	m := proposalMessage(p, s.botName)
	if kind == app.NotifyProposalAnswer {
		text := fmt.Sprintf("**Председатель ответил на ваше предложение.**\n%s\n\n%s", plain(p.Text), capitalizeRU(proposalStatusText[p.Status]))
		if p.Answer != "" {
			text += ": " + plain(p.Answer)
		}
		m = maxapi.NewMessage{Text: text, Format: "markdown", Attachments: []maxapi.Attachment{maxapi.Keyboard(
			[]maxapi.Button{maxapi.CallbackButton("Совет дома", pack(cbMenu, menuCouncil))},
		)}}
	}
	_, err := s.api.Send(ctx, maxapi.ToUser(maxUserID), m)
	return err
}
