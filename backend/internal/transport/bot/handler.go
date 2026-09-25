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
	"dommax/internal/app/cards"
	appcouncil "dommax/internal/app/council"
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
	cbHouse       = "h"    // h:<house_id> — выбрать дом
	cbNew         = "n"    // n:<category>:<text> — создать заявку
	cbPick        = "k"    // k:<text> — выбрать категорию
	cbJoin        = "j"    // j:<issue_id> — «это и у меня»
	cbConsent     = "c"    // c:<payload> — согласие, затем исходное действие
	cbConfirm     = "f"    // f:<issue_id> — «починили» в сообщении о выполнении
	cbRole        = "role" // role:<chairman|uk|resident> — сменить тестовую роль
	maxTextRunes  = 300    // длиннее — удобнее оформить в форме мини-приложения
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
	Cards          *cards.Service // карточка заявки для показа в чате
	Council        *appcouncil.Service
	Pending        app.BotPendingRepo // что бот ждёт от жителя следующим сообщением
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
	{Name: "menu", Description: "Главное меню"},
	{Name: "new", Description: "Сообщить о проблеме"},
	{Name: "my", Description: "Мои заявки"},
	{Name: "house", Description: "Мой дом и контакты УК"},
	{Name: "polls", Description: "Совет дома и опросы"},
	{Name: "role", Description: "Сменить роль (председатель, УК, житель)"},
	{Name: "help", Description: "Как это работает"},
}

const greetingText = "Здравствуйте! Я помогаю соседям сообщать о поломках в доме: лифт, свет в подъезде, протечка, отопление.\n\n" +
	"Одна заявка на весь дом вместо десятка сообщений в чате. Я покажу, кто отвечает и до какого срока, и напишу, когда статус изменится."

const helpText = "Напишите одним сообщением, что сломалось и где, например: «не горит свет на 5 этаже во втором подъезде». " +
	"Я определю категорию, ответственного и срок и проверю, не сообщали ли уже соседи.\n\n" +
	"Команды:\n/menu главное меню\n/new сообщить о проблеме\n/my мои заявки\n/house мой дом и контакты УК\n/polls совет дома и опросы\n\n" +
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
	// Альбом приходит одним сообщением с несколькими вложениями: берём все снимки.
	var urls []string
	for _, a := range m.Body.Attachments {
		switch a.Type {
		case "location":
			return h.onLocation(ctx, to, m.Sender, a.Latitude, a.Longitude)
		case "contact":
			return h.onContact(ctx, to, m.Sender, a)
		case "image":
			urls = append(urls, a.PhotoURL())
		}
	}
	if len(urls) > 0 {
		return h.onPhotos(ctx, to, m.Sender, urls)
	}
	txt := strings.TrimSpace(m.Body.Text)
	if txt == "" {
		return nil
	}
	if cmd, ok := strings.CutPrefix(txt, "/"); ok {
		cmd, arg, _ := strings.Cut(cmd, " ")
		switch cmd {
		case "new":
			return h.askProblem(ctx, to, m.Sender)
		case "help":
			return h.send(ctx, to, helpText)
		case "my":
			return h.myIssues(ctx, to, m.Sender)
		case "house":
			return h.myHouse(ctx, to, m.Sender)
		case "menu":
			return h.menu(ctx, to)
		case "polls":
			return h.councilMenu(ctx, to, m.Sender)
		case "role":
			return h.switchRole(ctx, to, m.Sender, arg)
		}
		return h.greet(ctx, to, "")
	}
	u, err := h.resident(ctx, m.Sender)
	if err != nil {
		return err
	}
	// Бот мог спросить комментарий или текст предложения: тогда это ответ, а не новая проблема.
	if handled, err := h.onPending(ctx, to, u, txt); handled || err != nil {
		return err
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
		if list, err := h.svc.Houses.Search(ctx, txt); err == nil && len(list) > 0 {
			var rows [][]maxapi.Button
			for _, hs := range list[:min(4, len(list))] {
				rows = append(rows, []maxapi.Button{maxapi.CallbackButton(hs.Address, pack(cbHouse, hs.ID))})
			}
			_, err := h.max.Send(ctx, to, maxapi.NewMessage{
				Text:        "По вашему адресу нашли в Москве:",
				Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)},
			})
			return err
		}
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

// askHouse спрашивает дом. Кнопки в два ряда: MAX добавляет к кнопке геопозиции иконку
// и обрезает текст, если две кнопки стоят рядом.
func (h *Handler) askHouse(ctx context.Context, to maxapi.Target) error {
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{
		Text: "Где вы живёте? Покажу дома рядом с вами по геопозиции. Или найдите свой дом по адресу в приложении.",
		Attachments: []maxapi.Attachment{maxapi.Keyboard(
			[]maxapi.Button{maxapi.GeoButton("Показать дома рядом")},
			[]maxapi.Button{maxapi.OpenAppButton("Найти дом по адресу", h.botName, "")},
		)},
	})
	return err
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
	if errors.Is(err, houses.ErrOutsideMoscow) {
		return h.outsideMoscowReply(ctx, to)
	}
	if err != nil {
		return err
	}
	if len(list) == 0 {
		// Точка в Москве, но дом по ней не определился (парк, стройка, геокодер не ответил).
		return h.sendKeyboard(ctx, to, "По этой точке дом не нашёлся. Найдите его по адресу в приложении.",
			[]maxapi.Button{maxapi.OpenAppButton("Найти дом по адресу", h.botName, "")})
	}
	var rows [][]maxapi.Button
	for _, hs := range list[:min(3, len(list))] {
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton(hs.Address, pack(cbHouse, hs.ID))})
	}
	_, err = h.max.Send(ctx, to, maxapi.NewMessage{Text: "Выберите свой дом:", Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}})
	return err
}

func (h *Handler) outsideMoscowReply(ctx context.Context, to maxapi.Target) error {
	sampleHouses, _ := h.svc.Houses.Search(ctx, "Ореховый")
	var rows [][]maxapi.Button
	for _, hs := range sampleHouses[:min(3, len(sampleHouses))] {
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton(hs.Address, pack(cbHouse, hs.ID))})
	}
	rows = append(rows, []maxapi.Button{maxapi.OpenAppButton("Найти московский дом", h.botName, "")})

	msg := "Вы находитесь за пределами Москвы. Сервис сейчас работает по всей Москве.\n\n" +
		"Вы можете найти московский дом по адресу или выбрать один из примеров для проверки:"
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{Text: msg, Attachments: []maxapi.Attachment{maxapi.Keyboard(rows...)}})
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
	return h.issueList(ctx, to, "Ваши заявки:", "Заявок пока нет. Напишите, что сломалось, и я помогу оформить заявку.", list)
}

// maxBotPhotos — сколько снимков из одного сообщения прикладывается за раз, как и в мини-приложении.
const maxBotPhotos = 3

// onPhotos прикрепляет присланные фото к последней открытой заявке жителя (FR-BOT-04).
// Ссылку на фото MAX отдаёт не во всех клиентах: тогда предлагаем добавить фото в карточке.
func (h *Handler) onPhotos(ctx context.Context, to maxapi.Target, from maxapi.User, urls []string) error {
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
	if !u.HasConsent(h.svc.ConsentVersion) {
		return h.sendKeyboard(ctx, to, "Чтобы приложить фото, нужно ваше согласие на обработку персональных данных. "+
			"Фото увидят только соседи, которые сообщили о проблеме, и УК. Данные хранятся в России.",
			[]maxapi.Button{maxapi.CallbackButton("Согласен", pack(cbConsent, ""))})
	}
	raws := make([][]byte, 0, min(len(urls), maxBotPhotos))
	for _, url := range urls[:min(len(urls), maxBotPhotos)] {
		if !strings.HasPrefix(url, "https://") {
			return h.sendKeyboard(ctx, to, "Не получилось получить фото. Добавьте его в карточке заявки.", open)
		}
		data, err := h.max.Download(ctx, url, photos.MaxBytes)
		if err != nil {
			h.log.WarnContext(ctx, "photo download failed", "err", err)
			return h.sendKeyboard(ctx, to, "Не получилось получить фото. Добавьте его в карточке заявки.", open)
		}
		raws = append(raws, data)
	}
	added, err := h.svc.Photos.Add(ctx, u, is.ID(), raws...)
	switch {
	case errors.Is(err, photos.ErrTooMany):
		return h.sendKeyboard(ctx, to, fmt.Sprintf("К заявке № %d можно приложить не больше %d фото.", is.Number(), photos.MaxPerIssue), open)
	case errors.Is(err, photos.ErrNotImage), errors.Is(err, photos.ErrTooLarge):
		return h.sendKeyboard(ctx, to, "Фото не подошло: нужен снимок JPEG или PNG до 5 МБ.", open)
	case err != nil:
		return err
	}
	done := fmt.Sprintf("Фото добавлено к заявке № %d. Его увидят соседи, которые сообщили о проблеме, и УК.", is.Number())
	if len(added) > 1 {
		done = fmt.Sprintf("%d фото добавлены к заявке № %d. Их увидят соседи, которые сообщили о проблеме, и УК.", len(added), is.Number())
	}
	return h.sendKeyboard(ctx, to, done, open)
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
		m := maxapi.NewMessage{
			Text:        fmt.Sprintf("Дом сохранён: %s.\nТеперь опишите проблему одним сообщением: что сломалось и где. Или выберите действие:", d.House.Address),
			Attachments: []maxapi.Attachment{menuKeyboard(h.botName)},
		}
		return maxapi.CallbackAnswer{Message: &m}, nil

	case cbMenu:
		return h.menuItem(ctx, maxapi.ToUser(cb.User.UserID), cb.User, rest)

	case cbIssue:
		if err := h.showIssue(ctx, maxapi.ToUser(cb.User.UserID), u, rest); err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		return maxapi.CallbackAnswer{Notification: "Заявка открыта ниже"}, nil

	case cbReopen:
		return h.askReopenComment(ctx, u, maxapi.ToUser(cb.User.UserID), rest)

	case cbVote:
		return h.vote(ctx, u, cb.Payload, rest)
	case cbCouncil:
		return h.councilItem(ctx, cb, u, rest)
	case cbAccept:
		return h.acceptProposal(ctx, u, rest)
	case cbDecline:
		return h.askDeclineAnswer(ctx, u, rest)
	case cbRole:
		return h.onRoleCallback(ctx, cb.User, rest)

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
		// Согласие перед фото: снимок в кнопку не положить, его нужно прислать снова.
		if rest == "" {
			return maxapi.CallbackAnswer{Notification: "Согласие сохранено. Пришлите фото ещё раз."}, nil
		}
		return h.callbackReply(ctx, &maxapi.Callback{ID: cb.ID, Payload: rest, User: cb.User})

	case cbConfirm:
		return h.confirmRepair(ctx, u, rest)

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
		// Лифт и свет бывают в каждом подъезде: сначала спросим, где именно.
		if answer, asked, err := h.askPlace(ctx, u, category, desc); asked || err != nil {
			return answer, err
		}
		return h.report(ctx, u, category, "", desc)

	case cbPlace:
		return h.placeChosen(ctx, u, rest)
	}
	return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела. Напишите о проблеме ещё раз."}, nil
}

// confirmRepair — «Починили» из сообщения о выполнении. Ошибки правил отвечают понятным уведомлением.
func (h *Handler) confirmRepair(ctx context.Context, u user.User, issueID string) (maxapi.CallbackAnswer, error) {
	_, err := h.svc.Issues.Confirm(ctx, u, issueID)
	switch {
	case err == nil:
		return maxapi.CallbackAnswer{Notification: "Спасибо, отметили: починили. Соседи и УК увидят это в заявке."}, nil
	case errors.Is(err, issue.ErrAlreadyAnswered):
		return maxapi.CallbackAnswer{Notification: "Вы уже ответили по этому ремонту."}, nil
	case errors.Is(err, issue.ErrWindowClosed):
		return maxapi.CallbackAnswer{Notification: "Прошло больше 7 дней после ремонта. Если проблема вернулась, сообщите о ней заново."}, nil
	case errors.Is(err, issue.ErrNotDone):
		return maxapi.CallbackAnswer{Notification: "Заявка снова в работе: ответить можно, когда УК отметит её выполненной."}, nil
	case errors.Is(err, issue.ErrNotParticipant), errors.Is(err, app.ErrNotFound):
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
	}
	return maxapi.CallbackAnswer{}, err
}

func (h *Handler) report(ctx context.Context, u user.User, category, objectID, desc string) (maxapi.CallbackAnswer, error) {
	is, err := h.svc.Issues.Report(ctx, u, issues.ReportInput{HouseID: u.HouseID, Category: category, ObjectID: objectID, Description: desc})
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	m := maxapi.NewMessage{Text: fmt.Sprintf("Заявка № %d отправлена в УК. Карточка со статусом придёт следующим сообщением и будет обновляться.", is.Number())}
	if !u.PhoneShared() {
		m.Text += "\n\nЕсли мастеру нужно попасть в квартиру, оставьте телефон: его увидит только УК, соседи номер не видят."
		m.Attachments = []maxapi.Attachment{maxapi.Keyboard([]maxapi.Button{maxapi.ContactButton("Оставить телефон для мастера")})}
	}
	return maxapi.CallbackAnswer{Message: &m}, nil
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

func (h *Handler) switchRole(ctx context.Context, to maxapi.Target, from maxapi.User, arg string) error {
	u, err := h.resident(ctx, from)
	if err != nil {
		return err
	}
	arg = strings.ToLower(strings.TrimSpace(arg))
	switch arg {
	case "chairman", "председатель":
		if u.HouseID == "" {
			return h.send(ctx, to, "Сначала выберите свой дом в /menu или отправьте геопозицию, чтобы стать председателем своего дома.")
		}
		if _, err := h.svc.Auth.SetRole(ctx, u, user.RoleResident, "", u.HouseID); err != nil {
			return err
		}
		return h.send(ctx, to, "Вам назначена роль: Председатель совета дома.\n\nТеперь в меню /polls вам доступна «Папка предложений», а при поступлении идей от соседей бот будет присылать их вам с кнопками ответа.")
	case "uk", "operator", "ук", "оператор":
		if _, err := h.svc.Auth.SwitchRole(ctx, u, "uk_operator"); err != nil {
			return err
		}
		return h.sendKeyboard(ctx, to, "Вам назначена роль: Сотрудник управляющей компании.\n\nОткройте мини-приложение, чтобы увидеть очередь заявок домов и дашборд метрик.", []maxapi.Button{
			maxapi.OpenAppButton("Открыть кабинет УК", h.botName, ""),
		})
	case "district", "район", "управа":
		u, err := h.svc.Auth.SwitchRole(ctx, u, "district")
		if err != nil {
			return err
		}
		return h.sendKeyboard(ctx, to, fmt.Sprintf("Вам назначена роль: Управа района (%s).\n\nОткройте мини-приложение, чтобы увидеть сравнение УК района и просроченные заявки.", u.District), []maxapi.Button{
			maxapi.OpenAppButton("Открыть кабинет района", h.botName, ""),
		})
	case "resident", "житель":
		if _, err := h.svc.Auth.SwitchRole(ctx, u, "resident"); err != nil {
			return err
		}
		return h.send(ctx, to, "Роль сброшена: обычный житель дома.")
	default:
		current := "житель"
		switch {
		case u.Role == user.RoleOperator:
			current = "сотрудник управляющей компании"
		case u.Role == user.RoleDistrict:
			current = "управа района"
		case u.ChairmanHouseID != "":
			current = "председатель совета дома"
		}
		msg := fmt.Sprintf("Ваша текущая роль: %s.\n\nВыберите роль для переключения:", current)
		_, err := h.max.Send(ctx, to, maxapi.NewMessage{
			Text: msg,
			Attachments: []maxapi.Attachment{maxapi.Keyboard(
				[]maxapi.Button{maxapi.CallbackButton("Стать председателем", pack(cbRole, "chairman")), maxapi.CallbackButton("Стать сотрудником УК", pack(cbRole, "uk"))},
				[]maxapi.Button{maxapi.CallbackButton("Управа района", pack(cbRole, "district")), maxapi.CallbackButton("Обычный житель", pack(cbRole, "resident"))},
			)},
		})
		return err
	}
}

func (h *Handler) onRoleCallback(ctx context.Context, from maxapi.User, role string) (maxapi.CallbackAnswer, error) {
	to := maxapi.ToUser(from.UserID)
	if err := h.switchRole(ctx, to, from, role); err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	return maxapi.CallbackAnswer{Notification: "Роль изменена"}, nil
}

func (h *Handler) greet(ctx context.Context, to maxapi.Target, startPayload string) error {
	_, err := h.max.Send(ctx, to, maxapi.NewMessage{
		Text:        greetingText,
		Attachments: []maxapi.Attachment{greetKeyboard(h.botName, startPayload)},
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
