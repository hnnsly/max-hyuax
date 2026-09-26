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
	m := withNav(pollMessage(v), scrCouncil)
	return maxapi.CallbackAnswer{Message: &m}, nil
}

// consentAnswer — просьба о согласии; после «Согласен» исходная кнопка выполнится сама.
func consentAnswer(text, payload string) maxapi.CallbackAnswer {
	m := maxapi.NewMessage{
		Text: text + " Данные хранятся в России.",
		Attachments: []maxapi.Attachment{maxapi.Keyboard(
			[]maxapi.Button{maxapi.CallbackButton("Согласен", pack(cbConsent, payload))},
			menuRow(),
		)},
	}
	return maxapi.CallbackAnswer{Message: &m}
}

// councilItem — действия совета, которые ждут текст: предложение и вопрос опроса.
// Списки открываются экранами (screens.go).
func (h *Handler) councilItem(ctx context.Context, cb *maxapi.Callback, u user.User, item string) (maxapi.CallbackAnswer, error) {
	var action, prompt string
	switch item {
	case councilMine:
		return h.openScreen(ctx, u, scrProps)
	case councilFolder:
		return h.openScreen(ctx, u, scrFolder)
	case councilPropose:
		if u.HouseID == "" {
			return h.openScreen(ctx, u, scrHome)
		}
		if !u.HasConsent(h.svc.ConsentVersion) {
			return consentAnswer("Чтобы написать совету, нужно ваше согласие на обработку персональных данных. Председатель увидит текст без вашего имени.", cb.Payload), nil
		}
		action, prompt = pendPropose, "Пожалуйста, опишите вашу идею или предложение по дому: что и где стоит улучшить. Председатель совета внимательно изучит его (анонимно, без вашего имени)."
	case councilNewPoll:
		if !u.IsChairmanOf(u.HouseID) {
			return maxapi.CallbackAnswer{Notification: "Создавать опросы может только председатель совета."}, nil
		}
		action, prompt = pendNewPoll, "Напишите вопрос опроса для жителей одним сообщением, например: «Установить шлагбаум на въезде во двор?»\n\nОпрос откроется на 7 дней с вариантами «За», «Против», «Воздержался»."
	default:
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
	}
	if err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: action, ExpiresAt: h.svc.Now().Add(pendingTTL)}); err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	m := screenMsg(prompt, navRow(scrCouncil))
	return maxapi.CallbackAnswer{Message: &m}, nil
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
	m, err := h.folderScreen(ctx, u)
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	return maxapi.CallbackAnswer{Message: &m, Notification: "Взято в работу, автор получит уведомление"}, nil
}

func (h *Handler) askDeclineAnswer(ctx context.Context, u user.User, id string) (maxapi.CallbackAnswer, error) {
	err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendDecline, Ref: id, ExpiresAt: h.svc.Now().Add(pendingTTL)})
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	m := screenMsg("Напишите одним сообщением, почему отклоняете. Автор увидит этот ответ.", navRow(scrProp, id))
	return maxapi.CallbackAnswer{Message: &m}, nil
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
		_, err = h.max.Send(ctx, to, withNav(pollMessage(v), scrCouncil))
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
			[]maxapi.Button{maxapi.CallbackButton("Совет дома", pack(cbMenu, scrCouncil))},
		)}}
	}
	_, err := s.api.Send(ctx, maxapi.ToUser(maxUserID), m)
	return err
}
