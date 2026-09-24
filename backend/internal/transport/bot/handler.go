// Пакет bot превращает события MAX в ответы бота. Long polling и webhook
// передают события в один и тот же Handler.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"dommax/internal/app"
	"dommax/internal/app/auth"
	"dommax/internal/app/hints"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/app/photos"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
	"dommax/internal/storage/maxapi"
)

// Payload callback-кнопок. Состояние диалога живёт в самих кнопках, поэтому бот
// не хранит черновики и переживает перезапуск.
const (
	PayloadReport = "report"
	cbHouse       = "h" // h:<house_id> — выбрать дом
	cbNew         = "n" // n:<category>:<text> — создать заявку
	cbPick        = "k" // k:<text> — выбрать категорию
	cbJoin        = "j" // j:<issue_id> — «это и у меня»
	cbConsent     = "c" // c:<payload> — согласие, затем исходное действие
	maxTextRunes  = 300 // длиннее — удобнее оформить в форме мини-приложения
)

// Messenger — часть Bot API, которая нужна обработчику.
type Messenger interface {
	Send(ctx context.Context, to maxapi.Target, m maxapi.NewMessage) (maxapi.Message, error)
	Answer(ctx context.Context, callbackID string, a maxapi.CallbackAnswer) error
	// Download скачивает присланный файл по ссылке из вложения.
	Download(ctx context.Context, url string, limit int64) ([]byte, error)
}

// Services — сценарии приложения, которыми пользуется диалог бота.
type Services struct {
	Auth           *auth.Service
	Issues         *issues.Service
	Houses         *houses.Service
	Hints          *hints.Service
	Photos         *photos.Service
	ConsentVersion string
	Now            func() time.Time
}

type Handler struct {
	max     Messenger
	botName string
	svc     Services
	log     *slog.Logger
}

// NewHandler принимает username бота: кнопки open_app запускают мини-приложение этого бота.
func NewHandler(m Messenger, botName string, svc Services, log *slog.Logger) *Handler {
	return &Handler{max: m, botName: botName, svc: svc, log: log}
}

// Команды для меню бота (PATCH /me/commands).
var Commands = []maxapi.Command{
	{Name: "new", Description: "Сообщить о проблеме"},
	{Name: "my", Description: "Мои заявки"},
	{Name: "house", Description: "Мой дом и контакты УК"},
	{Name: "help", Description: "Как это работает"},
}

const greetingText = "Здравствуйте! Я помогаю соседям сообщать о поломках в доме: лифт, свет в подъезде, протечка, отопление.\n\n" +
	"Одна заявка на весь дом вместо десятка сообщений в чате. Я покажу, кто отвечает и до какого срока, и напишу, когда статус изменится."

const helpText = "Напишите одним сообщением, что сломалось и где, например: «не горит свет на 5 этаже во втором подъезде». " +
	"Я определю категорию, ответственного и срок и проверю, не сообщали ли уже соседи.\n\n" +
	"Команды:\n/new сообщить о проблеме\n/my мои заявки\n/house мой дом и контакты УК\n\n" +
	"Соседи видят только число сообщивших. Имя получает только управляющая компания."

func (h *Handler) Handle(ctx context.Context, u maxapi.Update) error {
	switch u.Type {
	case maxapi.UpdateBotStarted:
		return h.greet(ctx, target(u.ChatID, u.User.UserID), u.Payload)
	case maxapi.UpdateMessageCreated:
		return h.onMessage(ctx, u.Message)
	case maxapi.UpdateMessageCallback:
		return h.onCallback(ctx, u.Callback)
	}
	return nil
}

func (h *Handler) onMessage(ctx context.Context, m *maxapi.Message) error {
	// Бот отвечает только в личных диалогах и не пишет в домовые чаты (требования MAX §1.5).
	if m == nil || m.Sender.IsBot || m.Recipient.ChatType != "dialog" {
		return nil
	}
	to := target(m.Recipient.ChatID, m.Sender.UserID)
	for _, a := range m.Body.Attachments {
		switch a.Type {
		case "location":
			return h.onLocation(ctx, to, m.Sender, a.Latitude, a.Longitude)
		case "image":
			return h.onPhoto(ctx, to, m.Sender, a.PhotoURL())
		}
	}
	txt := strings.TrimSpace(m.Body.Text)
	if txt == "" {
		return nil
	}
	if cmd, ok := strings.CutPrefix(txt, "/"); ok {
		cmd, _, _ = strings.Cut(cmd, " ")
		switch cmd {
		case "new":
			return h.askProblem(ctx, to, m.Sender)
		case "help":
			return h.send(ctx, to, helpText)
		case "my":
			return h.myIssues(ctx, to, m.Sender)
		case "house":
			return h.myHouse(ctx, to, m.Sender)
		}
		return h.greet(ctx, to, "")
	}
	return h.onProblemText(ctx, to, m.Sender, txt)
}

func (h *Handler) resident(ctx context.Context, mu maxapi.User) (user.User, error) {
	return h.svc.Auth.EnsureMaxUser(ctx, mu.UserID, mu.FirstName)
}

// onProblemText: дом → категория → похожая заявка или подтверждение.
func (h *Handler) onProblemText(ctx context.Context, to maxapi.Target, from maxapi.User, txt string) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	if u.HouseID == "" {
		return h.askHouse(ctx, to)
	}
	// Подсказка: модель (если подключена) или ключевые слова; житель подтверждает кнопкой.
	// Нераспознанный или длинный текст удобнее оформить в форме, категория подставится, если узнана.
	hint, ok, err := h.svc.Hints.Suggest(ctx, txt)
	if err != nil || !ok || utf8.RuneCountInString(txt) > maxTextRunes {
		return h.openForm(ctx, to, hint.Rule.Code)
	}
	rule := hint.Rule
	similar, err := h.svc.Issues.FindSimilar(ctx, u.HouseID, rule.Code, "")
	if err != nil {
		return err
	}
	if len(similar) > 0 {
		return h.offerJoin(ctx, to, similar[0], rule, txt)
	}
	return h.confirm(ctx, to, u.HouseID, rule, txt)
}

func (h *Handler) confirm(ctx context.Context, to maxapi.Target, houseID string, rule rules.Rule, txt string) error {
	d, err := h.svc.Houses.Get(ctx, houseID)
	if err != nil {
		return err
	}
	deadline := rule.Deadline(h.svc.Now().In(moscow))
	msg := fmt.Sprintf("Поняли так: %s.\nОтвечает %s, срок ответа до %s.\n\nОтправить заявку?",
		rule.Title, d.Organization.Name, dayMonth(deadline))
	return h.sendKeyboard(ctx, to, msg, []maxapi.Button{
		maxapi.CallbackButton("Отправить", pack(cbNew, rule.Code, txt)),
		maxapi.CallbackButton("Изменить", pack(cbPick, txt)),
	})
}

func (h *Handler) offerJoin(ctx context.Context, to maxapi.Target, is *issue.Issue, rule rules.Rule, txt string) error {
	n := is.ParticipantCount()
	msg := fmt.Sprintf("Об этом уже сообщили: «%s», заявка № %d. Сообщили соседи: %d.\n"+
		"Присоединитесь, чтобы УК видела, сколько человек ждёт ремонта. Отдельная заявка только замедлит ответ.",
		plain(is.Title()), is.Number(), n)
	return h.sendKeyboard(ctx, to, msg, []maxapi.Button{
		maxapi.CallbackButton("Это и у меня", pack(cbJoin, is.ID())),
		maxapi.CallbackButton("Это другое", pack(cbNew, rule.Code, txt)),
	})
}

func (h *Handler) openForm(ctx context.Context, to maxapi.Target, category string) error {
	payload := ""
	if category != "" {
		payload = "n_" + category
	}
	return h.sendKeyboard(ctx, to, "Эту проблему удобнее оформить в форме: там можно указать место и добавить подробности.",
		[]maxapi.Button{maxapi.OpenAppButton("Открыть форму", h.botName, payload)})
}

func (h *Handler) askHouse(ctx context.Context, to maxapi.Target) error {
	return h.sendKeyboard(ctx, to, "Где вы живёте? Отправьте геопозицию, и я покажу ближайшие дома. Или найдите дом по адресу в приложении.",
		[]maxapi.Button{maxapi.GeoButton("Отправить геопозицию"), maxapi.OpenAppButton("Найти по адресу", h.botName, "")})
}

func (h *Handler) askProblem(ctx context.Context, to maxapi.Target, from maxapi.User) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	if u.HouseID == "" {
		return h.askHouse(ctx, to)
	}
	return h.send(ctx, to, "Опишите проблему одним сообщением: что сломалось и где.")
}

func (h *Handler) onLocation(ctx context.Context, to maxapi.Target, from maxapi.User, lat, lon float64) error {
	if _, err := h.resident(ctx, from); err != nil {
		return err
	}
	list, err := h.svc.Houses.Nearest(ctx, lat, lon)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return h.send(ctx, to, "Рядом не нашлось домов из сервиса. Пока он работает в одном районе Москвы.")
	}
	var rows [][]maxapi.Button
	for _, hs := range list[:min(3, len(list))] {
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton(hs.Address, pack(cbHouse, hs.ID))})
	}
	_, err = h.max.Send(ctx, to, maxapi.NewMessage{Text: "Выберите свой дом:", Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}})
	return err
}

func (h *Handler) myIssues(ctx context.Context, to maxapi.Target, from maxapi.User) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	list, err := h.svc.Issues.Mine(ctx, u)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return h.send(ctx, to, "Заявок пока нет. Напишите, что сломалось, и я помогу оформить заявку.")
	}
	var rows [][]maxapi.Button
	for _, is := range list[:min(8, len(list))] {
		label := truncate(fmt.Sprintf("№ %d, %s", is.Number(), is.Title()), 60)
		rows = append(rows, []maxapi.Button{maxapi.OpenAppButton(label, h.botName, "i_"+is.ID())})
	}
	_, err = h.max.Send(ctx, to, maxapi.NewMessage{Text: "Ваши заявки:", Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}})
	return err
}

// onPhoto прикрепляет присланное фото к последней открытой заявке жителя (FR-BOT-04).
// Ссылку на фото MAX отдаёт не во всех клиентах: тогда предлагаем добавить фото в карточке.
func (h *Handler) onPhoto(ctx context.Context, to maxapi.Target, from maxapi.User, url string) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	list, err := h.svc.Issues.Mine(ctx, u)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(is *issue.Issue) bool { return !is.Status().Closed() })
	if i < 0 {
		return h.send(ctx, to, "Фото прикладывается к заявке. Сначала опишите проблему одним сообщением: что сломалось и где.")
	}
	is := list[i]
	open := []maxapi.Button{maxapi.OpenAppButton("Открыть заявку", h.botName, "i_"+is.ID())}
	if !strings.HasPrefix(url, "https://") {
		return h.sendKeyboard(ctx, to, "Не получилось получить фото. Добавьте его в карточке заявки.", open)
	}
	data, err := h.max.Download(ctx, url, photos.MaxBytes)
	if err != nil {
		h.log.WarnContext(ctx, "photo download failed", "err", err)
		return h.sendKeyboard(ctx, to, "Не получилось получить фото. Добавьте его в карточке заявки.", open)
	}
	_, err = h.svc.Photos.Add(ctx, u, is.ID(), data)
	switch {
	case errors.Is(err, photos.ErrTooMany):
		return h.sendKeyboard(ctx, to, fmt.Sprintf("К заявке № %d уже приложено %d фото, больше добавить нельзя.", is.Number(), photos.MaxPerIssue), open)
	case errors.Is(err, photos.ErrNotImage), errors.Is(err, photos.ErrTooLarge):
		return h.sendKeyboard(ctx, to, "Фото не подошло: нужен снимок JPEG или PNG до 5 МБ.", open)
	case err != nil:
		return err
	}
	return h.sendKeyboard(ctx, to, fmt.Sprintf("Фото добавлено к заявке № %d. Его увидят соседи, которые сообщили о проблеме, и УК.", is.Number()), open)
}

func (h *Handler) myHouse(ctx context.Context, to maxapi.Target, from maxapi.User) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	if u.HouseID == "" {
		return h.askHouse(ctx, to)
	}
	d, err := h.svc.Houses.Get(ctx, u.HouseID)
	if err != nil {
		return err
	}
	list, err := h.svc.Issues.ListByHouse(ctx, u.HouseID)
	if err != nil {
		return err
	}
	open := 0
	for _, is := range list {
		if !is.Status().Closed() {
			open++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Ваш дом: %s\nУК: %s\n", d.House.Address, d.Organization.Name)
	if p := d.Organization.PhoneDispatcher; p != "" {
		fmt.Fprintf(&b, "Диспетчерская: %s\n", p)
	}
	fmt.Fprintf(&b, "Открытых заявок: %d", open)
	return h.sendKeyboard(ctx, to, b.String(), []maxapi.Button{maxapi.OpenAppButton("Открыть дом", h.botName, "")})
}

// onCallback разбирает нажатие кнопки. Ответ заменяет сообщение с кнопками.
func (h *Handler) onCallback(ctx context.Context, cb *maxapi.Callback) error {
	if cb == nil {
		return nil
	}
	reply, err := h.callbackReply(ctx, cb)
	if err != nil {
		h.log.ErrorContext(ctx, "bot callback failed", "err", err)
		return h.max.Answer(ctx, cb.ID, maxapi.CallbackAnswer{Notification: "Не получилось. Попробуйте ещё раз."})
	}
	return h.max.Answer(ctx, cb.ID, reply)
}

func (h *Handler) callbackReply(ctx context.Context, cb *maxapi.Callback) (maxapi.CallbackAnswer, error) {
	if cb.Payload == PayloadReport {
		u, err := h.resident(ctx, cb.User)
		if err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		if u.HouseID == "" {
			// Кнопки геопозиции нельзя приложить к ответу-уведомлению: отправляем отдельным сообщением.
			return maxapi.CallbackAnswer{Notification: "Сначала укажите дом"}, h.askHouse(ctx, maxapi.ToUser(cb.User.UserID))
		}
		return maxapi.CallbackAnswer{Notification: "Опишите проблему одним сообщением: что сломалось и где."}, nil
	}

	kind, rest, _ := strings.Cut(cb.Payload, ":")
	u, err := h.resident(ctx, cb.User)
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	switch kind {
	case cbHouse:
		d, err := h.svc.Houses.Get(ctx, rest)
		if err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		if _, err := h.svc.Auth.SetHouse(ctx, u, rest); err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		return replace(fmt.Sprintf("Дом сохранён: %s.\nТеперь опишите проблему одним сообщением: что сломалось и где.", d.House.Address)), nil

	case cbPick:
		var rows [][]maxapi.Button
		for _, r := range rules.Categories() {
			rows = append(rows, []maxapi.Button{maxapi.CallbackButton(r.Title, pack(cbNew, r.Code, rest))})
		}
		m := maxapi.NewMessage{Text: "Выберите категорию:", Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}}
		return maxapi.CallbackAnswer{Message: &m}, nil

	case cbConsent:
		if _, err := h.svc.Auth.AcceptConsent(ctx, u, h.svc.ConsentVersion); err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		return h.callbackReply(ctx, &maxapi.Callback{ID: cb.ID, Payload: rest, User: cb.User})

	case cbNew, cbJoin:
		if !u.HasConsent(h.svc.ConsentVersion) {
			m := maxapi.NewMessage{
				Text: "Чтобы отправить заявку, нужно ваше согласие на обработку персональных данных. " +
					"Соседи увидят только число сообщивших, имя получит только управляющая компания. Данные хранятся в России.",
				Attachments: []maxapi.Attachment{maxapi.Keyboard([]maxapi.Button{maxapi.CallbackButton("Согласен", pack(cbConsent, cb.Payload))})},
			}
			return maxapi.CallbackAnswer{Message: &m}, nil
		}
		if kind == cbJoin {
			return h.join(ctx, u, rest)
		}
		category, desc, _ := strings.Cut(rest, ":")
		return h.report(ctx, u, category, desc)
	}
	return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела. Напишите о проблеме ещё раз."}, nil
}

func (h *Handler) report(ctx context.Context, u user.User, category, desc string) (maxapi.CallbackAnswer, error) {
	is, err := h.svc.Issues.Report(ctx, u, issues.ReportInput{HouseID: u.HouseID, Category: category, Description: desc})
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	return replace(fmt.Sprintf("Заявка № %d отправлена в УК. Карточка со статусом придёт следующим сообщением и будет обновляться.", is.Number())), nil
}

func (h *Handler) join(ctx context.Context, u user.User, issueID string) (maxapi.CallbackAnswer, error) {
	is, err := h.svc.Issues.Join(ctx, u, issueID)
	switch {
	case errors.Is(err, issue.ErrAlreadyJoined):
		return replace("Вы уже среди сообщивших по этой заявке."), nil
	case errors.Is(err, issue.ErrClosed):
		return replace("Эта заявка уже закрыта. Если проблема осталась, напишите о ней заново."), nil
	case errors.Is(err, app.ErrNotFound):
		return replace("Заявка не найдена. Напишите о проблеме заново."), nil
	case err != nil:
		return maxapi.CallbackAnswer{}, err
	}
	return replace(fmt.Sprintf("Вы присоединились к заявке № %d. Карточка со статусом придёт следующим сообщением.", is.Number())), nil
}

func (h *Handler) greet(ctx context.Context, to maxapi.Target, startPayload string) error {
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{
		Text: greetingText,
		Attachments: []maxapi.Attachment{maxapi.Keyboard(
			[]maxapi.Button{maxapi.CallbackButton("Сообщить о проблеме", PayloadReport)},
			[]maxapi.Button{maxapi.OpenAppButton("Открыть приложение", h.botName, startPayload)},
		)},
	})
	return err
}

func (h *Handler) send(ctx context.Context, to maxapi.Target, text string) error {
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{Text: text})
	return err
}

func (h *Handler) sendKeyboard(ctx context.Context, to maxapi.Target, text string, row []maxapi.Button) error {
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{Text: text, Attachments: []maxapi.Attachment{maxapi.Keyboard(row)}})
	return err
}

// replace — ответ на нажатие, который заменяет сообщение и убирает его кнопки.
func replace(text string) maxapi.CallbackAnswer {
	return maxapi.CallbackAnswer{Message: &maxapi.NewMessage{Text: text, Attachments: []maxapi.Attachment{}}}
}

// pack собирает payload кнопки; последний параметр может содержать двоеточия.
func pack(kind string, parts ...string) string {
	return truncate(kind+":"+strings.Join(parts, ":"), 1000)
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func target(chatID, userID int64) maxapi.Target {
	if chatID != 0 {
		return maxapi.ToChat(chatID)
	}
	return maxapi.ToUser(userID)
}
