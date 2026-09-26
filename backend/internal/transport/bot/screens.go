package bot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/issues"
	"dommax/internal/app/office"
	"dommax/internal/domain/council"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
	"dommax/internal/storage/maxapi"
)

// Одно окно (замечание команды 26.09): навигационные кнопки не присылают новое сообщение,
// а заменяют то, в котором нажаты. Меню, списки и карточки листаются на месте, как экраны
// приложения, и искать меню выше по чату не нужно. Новые сообщения бот шлёт только в ответ
// на текст и голос, в уведомлениях и по командам, и в конце у них всегда есть «Меню».

const (
	cbWin    = "w" // w:<экран>[:<аргументы>] — открыть экран в том же сообщении
	cbStatus = "u" // u:<issue_id>:<статус>[:c] — сотрудник УК меняет статус; c — без комментария
	cbSkip   = "x" // x:<категория>:<объект|-> — отправить заявку без описания
	cbRate   = "t" // t:<issue_id>:<1..5> — оценка ремонта после «Починили» (ADR-022)

	pendDescribe = "describe" // Ref — категория, Text — объект («-» — без объекта)
	pendStatus   = "status"   // Ref — заявка, Text — новый статус

	scrHome     = "home"
	scrMine     = "mine"
	scrNow      = "now"
	scrHouse    = "house"
	scrIssue    = "issue" // issue:<id>:<экран, куда вернуться>
	scrCouncil  = "council"
	scrPoll     = "poll" // poll:<id>
	scrProps    = "props"
	scrFolder   = "folder"
	scrProp     = "prop" // prop:<id>
	scrReport   = "report"
	scrPlace    = "place" // place:<категория>
	scrDescribe = "desc"  // desc:<категория>:<объект|->
	scrQueue    = "queue"
	scrMetrics  = "metrics"
	scrDistrict = "district"
	scrOverdue  = "overdue"
	scrRole     = "role"
	scrRating   = "rating"
	scrAlerts   = "alerts"
	scrVisit    = "visit"

	// queueInChat — сколько заявок очереди УК показывать кнопками; остальные в кабинете.
	queueInChat = 10
)

var statusButton = map[issue.Status]string{
	issue.StatusAccepted:   "Принять",
	issue.StatusInProgress: "В работу",
	issue.StatusDone:       "Выполнено",
	issue.StatusRejected:   "Отклонить",
}

// win — payload кнопки перехода на экран.
func win(name string, args ...string) string { return pack(cbWin, append([]string{name}, args...)...) }

// navRow — «Назад» (если есть куда) и «Меню» внизу каждого экрана.
func navRow(back ...string) []maxapi.Button {
	var row []maxapi.Button
	if len(back) > 0 && back[0] != "" && back[0] != scrHome {
		row = append(row, maxapi.CallbackButton("Назад", win(back[0], back[1:]...)))
	}
	return append(row, maxapi.CallbackButton("Меню", win(scrHome)))
}

// menuRow — одна кнопка «Меню» под ответами бота; меню открывается на месте ответа.
func menuRow() []maxapi.Button {
	return []maxapi.Button{maxapi.CallbackButton("Меню", win(scrHome))}
}

// menuBelowRow — «Меню» в уведомлениях и живой карточке: меню приходит новым сообщением,
// а уведомление остаётся на месте, и живую карточку бот продолжает обновлять.
func menuBelowRow() []maxapi.Button {
	return []maxapi.Button{maxapi.CallbackButton("Меню", pack(cbMenu, scrHome))}
}

func screenMsg(text string, rows ...[]maxapi.Button) maxapi.NewMessage {
	m := maxapi.NewMessage{Text: text, Format: "markdown"}
	if len(rows) > 0 {
		m.Attachments = []maxapi.Attachment{maxapi.Keyboard(rows...)}
	}
	return m
}

// openScreen — ответ на навигационное нажатие: экран заменяет сообщение с кнопкой.
func (h *Handler) openScreen(ctx context.Context, u user.User, rest string) (maxapi.CallbackAnswer, error) {
	name, arg, _ := strings.Cut(rest, ":")
	m, err := h.screen(ctx, u, name, arg)
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	return maxapi.CallbackAnswer{Message: &m}, nil
}

// sendScreen — тот же экран новым сообщением внизу чата (команды и кнопки уведомлений).
func (h *Handler) sendScreen(ctx context.Context, to maxapi.Target, u user.User, name, arg string) error {
	m, err := h.screen(ctx, u, name, arg)
	if err != nil {
		return err
	}
	_, err = h.max.Send(ctx, to, m)
	return err
}

// screen собирает экран по имени. Переход на экран отменяет ожидание текста: «Назад» после
// вопроса бота не должен превратить следующее сообщение в ответ на этот вопрос.
func (h *Handler) screen(ctx context.Context, u user.User, name, arg string) (maxapi.NewMessage, error) {
	h.dropPending(ctx, u)
	switch name {
	case scrMine:
		list, err := h.svc.Issues.Mine(ctx, u)
		if err != nil {
			return maxapi.NewMessage{}, err
		}
		return issueListScreen("Ваши заявки:", "Заявок пока нет. Нажмите «Сообщить о проблеме» или напишите, что сломалось.", list, scrMine, scrHome), nil
	case scrNow:
		return h.houseNowScreen(ctx, u)
	case scrHouse:
		return h.houseScreen(ctx, u)
	case scrIssue:
		id, back, _ := strings.Cut(arg, ":")
		return h.issueScreen(ctx, u, id, back)
	case scrCouncil:
		return h.councilScreen(ctx, u)
	case scrPoll:
		return h.pollScreen(ctx, u, arg)
	case scrProps:
		return h.propsScreen(ctx, u)
	case scrFolder:
		return h.folderScreen(ctx, u)
	case scrProp:
		return h.propScreen(ctx, u, arg)
	case scrReport:
		return h.reportScreen(ctx, u)
	case scrPlace:
		return h.placeScreen(ctx, u, arg)
	case scrDescribe:
		category, object, _ := strings.Cut(arg, ":")
		return h.describeScreen(ctx, u, category, object)
	case scrQueue:
		return h.queueScreen(ctx, u)
	case scrMetrics:
		return h.metricsScreen(ctx, u)
	case scrDistrict:
		return h.districtScreen(ctx, u)
	case scrOverdue:
		return h.overdueScreen(ctx, u)
	case scrRole:
		return h.roleScreen(u), nil
	case scrRating:
		return h.ratingScreen(ctx, u)
	case scrAlerts:
		return h.alertsScreen(ctx, u)
	case scrVisit:
		return h.visitScreen(ctx, u)
	}
	return h.homeScreen(ctx, u)
}

// homeScreen — главное меню по роли: у жителя, председателя, сотрудника УК и управы свои пункты.
func (h *Handler) homeScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	var roleRow []maxapi.Button
	if h.svc.RoleSwitch {
		roleRow = []maxapi.Button{maxapi.CallbackButton("Роль для проверки", win(scrRole))}
	}
	withRole := func(rows ...[]maxapi.Button) [][]maxapi.Button {
		if roleRow != nil {
			rows = append(rows, roleRow)
		}
		return rows
	}
	switch u.Role {
	case user.RoleOperator:
		return screenMsg("**Кабинет управляющей компании.** Очередь заявок по срочности и метрики.", withRole(
			[]maxapi.Button{maxapi.CallbackButton("Очередь заявок", win(scrQueue))},
			[]maxapi.Button{maxapi.CallbackButton("Метрики", win(scrMetrics))},
			[]maxapi.Button{maxapi.OpenAppButton("Открыть кабинет УК", h.botName, "")},
		)...), nil
	case user.RoleDistrict:
		return screenMsg(fmt.Sprintf("**Управа района %s.** Сравнение управляющих компаний и просроченные заявки.", plain(u.District)), withRole(
			[]maxapi.Button{maxapi.CallbackButton("Сводка района", win(scrDistrict))},
			[]maxapi.Button{maxapi.CallbackButton("Просрочено в районе", win(scrOverdue))},
			[]maxapi.Button{maxapi.OpenAppButton("Открыть кабинет района", h.botName, "")},
		)...), nil
	}
	if u.HouseID == "" {
		return screenMsg("Сначала выберите свой дом: покажу дома рядом по геопозиции или найдите дом по адресу в приложении.",
			[]maxapi.Button{maxapi.GeoButton("Показать дома рядом")},
			[]maxapi.Button{maxapi.OpenAppButton("Найти дом по адресу", h.botName, "")},
		), nil
	}
	address := ""
	if d, err := h.svc.Houses.Get(ctx, u.HouseID); err == nil {
		address = plain(d.House.Address)
	}
	rows := [][]maxapi.Button{
		{maxapi.CallbackButton("Сообщить о проблеме", win(scrReport))},
		{maxapi.CallbackButton("Мои заявки", win(scrMine)), maxapi.CallbackButton("Сейчас в доме", win(scrNow))},
		{maxapi.CallbackButton("Совет дома", win(scrCouncil)), maxapi.CallbackButton("Мой дом", win(scrHouse))},
	}
	if u.IsChairmanOf(u.HouseID) {
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Папка предложений", win(scrFolder))})
	}
	rows = append(rows,
		[]maxapi.Button{maxapi.CallbackButton("Плановые работы", win(scrAlerts)), maxapi.CallbackButton("Приём в УК", win(scrVisit))},
		[]maxapi.Button{maxapi.CallbackButton("Рейтинг УК района", win(scrRating))},
		[]maxapi.Button{maxapi.OpenAppButton("Открыть приложение", h.botName, "")},
	)
	return screenMsg(fmt.Sprintf("**Меню.** Ваш дом: %s.\nВыберите действие или просто напишите или наговорите проблему одним сообщением.", address), withRole(rows...)...), nil
}

// issueListScreen — заявки кнопками; нажатие открывает карточку в том же сообщении.
func issueListScreen(title, empty string, list []*issue.Issue, from, back string) maxapi.NewMessage {
	if len(list) == 0 {
		return screenMsg(empty, navRow(back))
	}
	var rows [][]maxapi.Button
	for _, is := range list[:min(listLimit, len(list))] {
		label := truncate(fmt.Sprintf("№ %d, %s", is.Number(), is.Title()), 60)
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton(label, win(scrIssue, is.ID(), from))})
	}
	return screenMsg(title, append(rows, navRow(back))...)
}

func (h *Handler) houseNowScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	if u.HouseID == "" {
		return h.homeScreen(ctx, u)
	}
	list, err := h.svc.Issues.ListByHouse(ctx, u.HouseID)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var open []*issue.Issue
	for _, is := range list {
		if !is.Status().Closed() {
			open = append(open, is)
		}
	}
	return issueListScreen("Сейчас в доме открыты заявки. Если у вас то же самое, откройте заявку и присоединитесь:",
		"В доме нет открытых заявок. Если что-то сломалось, нажмите «Сообщить о проблеме».", open, scrNow, scrHome), nil
}

func (h *Handler) houseScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	if u.HouseID == "" {
		return h.homeScreen(ctx, u)
	}
	d, err := h.svc.Houses.Get(ctx, u.HouseID)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	list, err := h.svc.Issues.ListByHouse(ctx, u.HouseID)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	open := 0
	for _, is := range list {
		if !is.Status().Closed() {
			open++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Ваш дом:** %s\nУК: %s\n", plain(d.House.Address), plain(d.Organization.Name))
	if p := d.Organization.PhoneDispatcher; p != "" {
		fmt.Fprintf(&b, "Диспетчерская: %s\n", p)
	}
	fmt.Fprintf(&b, "Открытых заявок: %d", open)
	rows := [][]maxapi.Button{
		{maxapi.OpenAppButton("Открыть дом в приложении", h.botName, "")},
	}
	if u.PhoneShared() {
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Не показывать телефон мастеру", pack(cbHidePhone, ""))})
	}
	rows = append(rows, navRow())
	return screenMsg(b.String(), rows...), nil
}

// issueScreen — карточка заявки с кнопками по её состоянию и роли: житель присоединяется
// и проверяет ремонт, сотрудник ответственной УК меняет статус.
func (h *Handler) issueScreen(ctx context.Context, u user.User, issueID, back string) (maxapi.NewMessage, error) {
	is, err := h.svc.Issues.Get(ctx, issueID)
	if errors.Is(err, app.ErrNotFound) {
		return screenMsg("Заявка не найдена.", navRow(back)), nil
	}
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	c, err := h.svc.Cards.Card(ctx, issueID)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	m := RenderCard(c, h.botName)
	text := m.Text
	if events, err := h.svc.Issues.Timeline(ctx, issueID); err == nil && len(events) > 0 {
		var b strings.Builder
		b.WriteString(text + "\n\n**Хронология**")
		for _, e := range events[max(0, len(events)-3):] {
			fmt.Fprintf(&b, "\n%s: %s", dayMonth(e.At), eventLine(e))
		}
		text = b.String()
	}

	var rows [][]maxapi.Button
	open := !is.Status().Closed()
	switch {
	case u.CanManageIssues(is.ResponsibleOrgID()) && open:
		var row []maxapi.Button
		for _, st := range issue.NextStatuses(is.Status()) {
			row = append(row, maxapi.CallbackButton(statusButton[st], pack(cbStatus, is.ID(), string(st))))
		}
		rows = append(rows, row)
	case repairAsk(is, u, h.svc.Now()):
		rows = append(rows, []maxapi.Button{
			maxapi.CallbackButton("Починили", ConfirmPayload(is.ID())),
			maxapi.CallbackButton("Не починили", pack(cbReopen, is.ID())),
		})
	case rateAsk(is, u, h.svc.Now()):
		text += "\n\nОцените ремонт от 1 до 5:"
		rows = append(rows, starsRow(is.ID()))
	case open && !is.HasParticipant(u.ID) && u.CanTakePart() && u.HouseID == is.HouseID():
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Это и у меня", pack(cbJoin, is.ID()))})
	case open && is.HasParticipant(u.ID) && !u.PhoneShared():
		rows = append(rows, []maxapi.Button{maxapi.ContactButton("Оставить телефон для мастера")})
	case is.HasParticipant(u.ID) && u.PhoneShared():
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Не показывать телефон мастеру", pack(cbHidePhone, is.ID()))})
	}
	if open && is.HasParticipant(u.ID) && (is.IsOverdue(h.svc.Now()) || !is.OverdueAt().IsZero()) && h.svc.Appeal != nil {
		if sum, err := h.svc.Appeal.Summary(ctx, u, is.ID()); err == nil {
			if sum.Mine {
				rows = append(rows, []maxapi.Button{
					maxapi.CallbackButton(fmt.Sprintf("Отозвать подпись ГЖИ (%d)", sum.Count), pack(cbWithdrawAppeal, is.ID())),
				})
			} else {
				rows = append(rows, []maxapi.Button{
					maxapi.CallbackButton(fmt.Sprintf("Подписать обращение в ГЖИ (%d)", sum.Count), pack(cbSignAppeal, is.ID())),
				})
			}
		}
	}
	rows = append(rows,
		[]maxapi.Button{
			maxapi.OpenAppButton("Открыть заявку", h.botName, "i_"+is.ID()),
			maxapi.LinkButton("Поделиться", shareURL(c, h.botName)),
		},
		navRow(back),
	)
	return screenMsg(text, rows...), nil
}

func (h *Handler) councilScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	if u.HouseID == "" {
		return h.homeScreen(ctx, u)
	}
	polls, err := h.svc.Council.Polls(ctx, u)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var rows [][]maxapi.Button
	for _, p := range polls {
		if p.Open && len(rows) < pollsInChat {
			rows = append(rows, []maxapi.Button{maxapi.CallbackButton(truncate("Опрос: "+p.Question, 60), win(scrPoll, p.ID))})
		}
	}
	text := "**Совет дома.** Предложения председателю и опросы соседей без юридической силы."
	if len(rows) == 0 {
		text += "\nОткрытых опросов сейчас нет."
	}
	rows = append(rows,
		[]maxapi.Button{maxapi.CallbackButton("Предложить совету", pack(cbCouncil, councilPropose))},
		[]maxapi.Button{maxapi.CallbackButton("Мои предложения", win(scrProps))},
	)
	if u.IsChairmanOf(u.HouseID) {
		rows = append(rows, []maxapi.Button{
			maxapi.CallbackButton("Папка предложений", win(scrFolder)),
			maxapi.CallbackButton("Создать опрос", pack(cbCouncil, councilNewPoll)),
		})
	}
	return screenMsg(text, append(rows, navRow())...), nil
}

func (h *Handler) pollScreen(ctx context.Context, u user.User, pollID string) (maxapi.NewMessage, error) {
	polls, err := h.svc.Council.Polls(ctx, u)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	for _, p := range polls {
		if p.ID == pollID {
			m := pollMessage(p)
			return withNav(m, scrCouncil), nil
		}
	}
	return screenMsg("Опрос не найден.", navRow(scrCouncil)), nil
}

// withNav добавляет ряд «Назад» и «Меню» к сообщению с клавиатурой или без неё.
func withNav(m maxapi.NewMessage, back ...string) maxapi.NewMessage {
	var rows [][]maxapi.Button
	for _, a := range m.Attachments {
		if kb, ok := a.Payload.(maxapi.KeyboardPayload); ok {
			rows = append(rows, kb.Buttons...)
		}
	}
	m.Attachments = []maxapi.Attachment{maxapi.Keyboard(append(rows, navRow(back...))...)}
	return m
}

func (h *Handler) propsScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	list, err := h.svc.Council.MyProposals(ctx, u)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	if len(list) == 0 {
		return screenMsg("Предложений пока нет. Нажмите «Предложить совету» в разделе «Совет дома».", navRow(scrCouncil)), nil
	}
	var b strings.Builder
	b.WriteString("**Мои предложения**")
	for i, p := range list[:min(listLimit, len(list))] {
		fmt.Fprintf(&b, "\n\n%d. %s\n%s", i+1, plain(truncate(p.Text, 200)), capitalizeRU(proposalStatusText[p.Status]))
		if p.Answer != "" {
			fmt.Fprintf(&b, ": %s", plain(p.Answer))
		}
	}
	return screenMsg(b.String(), navRow(scrCouncil)), nil
}

func (h *Handler) folderScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	list, err := h.svc.Council.Folder(ctx, u)
	if errors.Is(err, app.ErrForbidden) {
		return screenMsg("Папка доступна председателю совета дома.", navRow(scrCouncil)), nil
	}
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var rows [][]maxapi.Button
	for _, p := range list {
		if p.Status == council.StatusNew && len(rows) < listLimit {
			rows = append(rows, []maxapi.Button{maxapi.CallbackButton(truncate(p.Text, 60), win(scrProp, p.ID))})
		}
	}
	text := "**Папка предложений.** Новые предложения соседей, имена авторов не показываются."
	if len(rows) == 0 {
		text = "Новых предложений нет."
	}
	return screenMsg(text, append(rows, navRow(scrCouncil))...), nil
}

func (h *Handler) propScreen(ctx context.Context, u user.User, id string) (maxapi.NewMessage, error) {
	list, err := h.svc.Council.Folder(ctx, u)
	if err != nil {
		return screenMsg("Папка доступна председателю совета дома.", navRow(scrCouncil)), nil
	}
	for _, p := range list {
		if p.ID == id {
			m := proposalMessage(p, h.botName)
			return withNav(m, scrFolder), nil
		}
	}
	return screenMsg("Предложение не найдено.", navRow(scrFolder)), nil
}

// reportScreen — первый шаг заявки кнопками: категория.
func (h *Handler) reportScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	if u.HouseID == "" || !u.CanTakePart() {
		return h.homeScreen(ctx, u)
	}
	var hasElevator = true
	if d, err := h.svc.Houses.Get(ctx, u.HouseID); err == nil {
		if d.House.Floors > 0 && d.House.Floors <= 5 {
			hasElevator = false
			for _, o := range d.Objects {
				if o.Category == "lift" {
					hasElevator = true
					break
				}
			}
		}
	}
	var filtered []rules.Rule
	for _, c := range rules.Categories() {
		if c.Code == "lift" && !hasElevator {
			continue
		}
		filtered = append(filtered, c)
	}
	var rows [][]maxapi.Button
	for i := 0; i < len(filtered); i += 2 {
		row := []maxapi.Button{maxapi.CallbackButton(filtered[i].Title, win(scrPlace, filtered[i].Code))}
		if i+1 < len(filtered) {
			row = append(row, maxapi.CallbackButton(filtered[i+1].Title, win(scrPlace, filtered[i+1].Code)))
		}
		rows = append(rows, row)
	}
	return screenMsg("**Что случилось?** Выберите категорию или просто напишите проблему одним сообщением.", append(rows, navRow())...), nil
}

// placeScreen — второй шаг: подъезд, если у дома несколько объектов этой категории.
func (h *Handler) placeScreen(ctx context.Context, u user.User, category string) (maxapi.NewMessage, error) {
	if u.HouseID == "" {
		return h.homeScreen(ctx, u)
	}
	d, err := h.svc.Houses.Get(ctx, u.HouseID)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var rows [][]maxapi.Button
	for _, o := range d.Objects {
		if o.Category == category && o.EntranceID != "" {
			rows = append(rows, []maxapi.Button{maxapi.CallbackButton(truncate(capitalizeRU(o.Label), 40), win(scrDescribe, category, o.ID))})
		}
	}
	if len(rows) < 2 {
		return h.describeScreen(ctx, u, category, "-")
	}
	rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Не знаю или во всём доме", win(scrDescribe, category, "-"))})
	return screenMsg("**Где именно?** Так УК быстрее найдёт место.", append(rows, navRow(scrReport))...), nil
}

// describeScreen — третий шаг: кто отвечает и до какого срока, описание текстом или голосом.
// Описание ждёт в app.BotPending: следующее сообщение жителя отправит заявку.
func (h *Handler) describeScreen(ctx context.Context, u user.User, category, object string) (maxapi.NewMessage, error) {
	rule, err := rules.Lookup(category)
	if err != nil {
		return screenMsg("Такой категории нет. Выберите другую.", navRow(scrReport)), nil
	}
	d, err := h.svc.Houses.Get(ctx, u.HouseID)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	objectID := object
	if object == "-" {
		objectID = ""
	}
	place := ""
	for _, o := range d.Objects {
		if o.ID == objectID {
			place = ", " + o.Label
		}
	}
	if !u.HasConsent(h.svc.ConsentVersion) {
		return screenMsg("Чтобы отправить заявку, нужно ваше согласие на обработку персональных данных. "+
			"Соседи увидят только число сообщивших, имя получит только управляющая компания. Данные хранятся в России.",
			[]maxapi.Button{maxapi.CallbackButton("Согласен", pack(cbConsent, win(scrDescribe, category, object)))},
			navRow(scrReport),
		), nil
	}
	err = h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendDescribe, Ref: category, Text: object, ExpiresAt: h.svc.Now().Add(pendingTTL)})
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var b strings.Builder
	var rows [][]maxapi.Button
	if similar, err := h.svc.Issues.FindSimilar(ctx, u.HouseID, category, objectID); err == nil && len(similar) > 0 {
		is := similar[0]
		fmt.Fprintf(&b, "Похоже, об этом уже сообщили: заявка № %d «%s», сообщили соседи: %d. Если это то же самое, присоединитесь.\n\n",
			is.Number(), plain(is.Title()), is.ParticipantCount())
		rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Это и у меня", pack(cbJoin, is.ID()))})
	}
	fmt.Fprintf(&b, "**%s**%s\nОтвечает %s, срок ответа до %s.\n\nОпишите в двух словах, что случилось: текстом или голосовым сообщением. Или отправьте без описания.",
		rule.Title, plain(place), plain(d.Organization.Name), dayMonth(rule.Deadline(h.svc.Now().In(moscow))))
	rows = append(rows, []maxapi.Button{maxapi.CallbackButton("Отправить без описания", pack(cbSkip, category, object))})
	return screenMsg(b.String(), append(rows, navRow(scrPlace, category))...), nil
}

// maxDescRunes — описание из чата длиннее этого обрезается: подробности удобнее дописать в приложении.
const maxDescRunes = 1000

// reportMessage отправляет заявку, собранную кнопками; ответ — номер заявки и «Меню».
func (h *Handler) reportMessage(ctx context.Context, u user.User, category, object, desc string) (maxapi.NewMessage, error) {
	if object == "-" {
		object = ""
	}
	a, err := h.report(ctx, u, category, object, truncate(desc, maxDescRunes))
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	return *a.Message, nil
}

// statusComment — комментарий сотрудника УК к «Выполнено» или причина отказа, которых ждал бот.
func (h *Handler) statusComment(ctx context.Context, to maxapi.Target, u user.User, p app.BotPending, txt string) error {
	back := navRow(scrIssue, p.Ref, scrQueue)
	is, err := h.svc.Issues.ChangeStatus(ctx, u, p.Ref, issue.Status(p.Text), txt)
	if err != nil {
		a, err := statusError(err)
		if err != nil {
			return err
		}
		_, err = h.max.Send(ctx, to, screenMsg(a.Notification, back))
		return err
	}
	_, err = h.max.Send(ctx, to, screenMsg(
		fmt.Sprintf("Заявка № %d: статус «%s». Жители получат уведомление с вашим комментарием.", is.Number(), statusWord[is.Status()]),
		[]maxapi.Button{maxapi.CallbackButton("Открыть заявку", win(scrIssue, p.Ref, scrQueue)), maxapi.CallbackButton("Очередь", win(scrQueue))},
		menuRow(),
	))
	return err
}

func (h *Handler) queueScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	list, err := h.svc.Issues.Queue(ctx, u)
	if errors.Is(err, app.ErrForbidden) {
		return screenMsg("Очередь доступна сотруднику управляющей компании.", navRow()), nil
	}
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	now := h.svc.Now()
	var rows [][]maxapi.Button
	open, overdue := 0, 0
	for _, is := range list {
		if is.Status().Closed() {
			continue
		}
		open++
		when := "до " + dayMonth(is.Deadline())
		if is.IsOverdue(now) {
			overdue++
			when = "просрочено"
		}
		if len(rows) < queueInChat {
			label := truncate(fmt.Sprintf("№ %d %s, %s", is.Number(), is.Title(), when), 60)
			rows = append(rows, []maxapi.Button{maxapi.CallbackButton(label, win(scrIssue, is.ID(), scrQueue))})
		}
	}
	text := fmt.Sprintf("**Очередь заявок.** Открыто %d, из них просрочено %d. Сначала самые срочные.", open, overdue)
	if open == 0 {
		text = "Открытых заявок нет."
	}
	return screenMsg(text, append(rows, navRow())...), nil
}

func (h *Handler) metricsScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	m, err := h.svc.Issues.Metrics(ctx, u)
	if errors.Is(err, app.ErrForbidden) {
		return screenMsg("Метрики доступны сотруднику управляющей компании.", navRow()), nil
	}
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var b strings.Builder
	b.WriteString("**Метрики за 30 дней**\n")
	fmt.Fprintf(&b, "Подано заявок: %d, закрыто: %d", m.Issues, m.ClosedTotal)
	if m.ClosedTotal > 0 {
		fmt.Fprintf(&b, ", в срок %d%%", m.ClosedOnTime*100/m.ClosedTotal)
	}
	fmt.Fprintf(&b, ".\nОткрыто сейчас: %d, просрочено: %d.\n", m.OpenTotal, m.OverdueOpen)
	if m.Week != nil {
		fmt.Fprintf(&b, "Первый ответ жителю: медиана %s за неделю.\n", durationRU(*m.Week))
	}
	fmt.Fprintf(&b, "Жители подтвердили ремонт: %d, вернули в работу: %d.", m.Confirmed, m.Reopened)
	return screenMsg(b.String(), []maxapi.Button{maxapi.OpenAppButton("Графики в кабинете", h.botName, "")}, navRow()), nil
}

func (h *Handler) districtScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	d, err := h.svc.Issues.DistrictMetrics(ctx, u)
	if errors.Is(err, app.ErrForbidden) {
		return screenMsg("Сводка доступна управе района.", navRow()), nil
	}
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Район %s: управляющие компании за 30 дней**", plain(d.District))
	for _, o := range d.Orgs {
		fmt.Fprintf(&b, "\n\n%s\nОткрыто %d, просрочено %d", plain(o.Org.Name), o.OpenTotal, o.OverdueOpen)
		if o.ClosedTotal > 0 {
			fmt.Fprintf(&b, ", закрыто в срок %d%%", o.ClosedOnTime*100/o.ClosedTotal)
		}
	}
	return screenMsg(b.String(),
		[]maxapi.Button{maxapi.CallbackButton("Просрочено в районе", win(scrOverdue))},
		navRow(),
	), nil
}

func (h *Handler) overdueScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	list, err := h.svc.Issues.DistrictOverdue(ctx, u)
	if errors.Is(err, app.ErrForbidden) {
		return screenMsg("Список доступен управе района.", navRow()), nil
	}
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	return issueListScreen("**Просрочено в районе**, самые давние первыми:", "Просроченных заявок в районе нет.", list, scrOverdue, scrDistrict), nil
}

// roleSwitchOff — ответ, когда роль для проверки выключена (ROLE_SWITCH_ENABLED=false).
const roleSwitchOff = "Смена роли для проверки на этом сервере выключена. Ваша роль: житель."

// roleScreen — выбор роли для проверки.
func (h *Handler) roleScreen(u user.User) maxapi.NewMessage {
	if !h.svc.RoleSwitch {
		return screenMsg(roleSwitchOff, navRow())
	}
	current := "житель"
	switch {
	case u.Role == user.RoleOperator:
		current = "сотрудник управляющей компании"
	case u.Role == user.RoleDistrict:
		current = "управа района"
	case u.ChairmanHouseID != "":
		current = "председатель совета дома"
	}
	return screenMsg(fmt.Sprintf("**Роль для проверки.** Сейчас: %s. Данные настоящих жителей в других ролях не показываются.", current),
		[]maxapi.Button{maxapi.CallbackButton("Председатель", pack(cbRole, "chairman")), maxapi.CallbackButton("Сотрудник УК", pack(cbRole, "uk"))},
		[]maxapi.Button{maxapi.CallbackButton("Управа района", pack(cbRole, "district")), maxapi.CallbackButton("Житель", pack(cbRole, "resident"))},
		navRow(),
	)
}

// rateAsk — житель подтвердил ремонт, но ещё не оценил его, и 7 дней после «выполнено» не прошли.
func rateAsk(is *issue.Issue, u user.User, now time.Time) bool {
	a, ok := is.AnswerOf(u.ID)
	return ok && a.Fixed && a.Stars == 0 && now.Before(is.AnswerUntil())
}

// starsRow — оценка ремонта кнопками от 1 до 5.
func starsRow(issueID string) []maxapi.Button {
	row := make([]maxapi.Button, 0, 5)
	for n := range 5 {
		s := strconv.Itoa(n + 1)
		row = append(row, maxapi.CallbackButton(s, pack(cbRate, issueID, s)))
	}
	return row
}

// rate — оценка ремонта кнопкой после «Починили» (ADR-022).
func (h *Handler) rate(ctx context.Context, u user.User, rest string) (maxapi.CallbackAnswer, error) {
	id, n, _ := strings.Cut(rest, ":")
	stars, err := strconv.Atoi(n)
	if err != nil {
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
	}
	_, err = h.svc.Issues.Rate(ctx, u, id, stars)
	switch {
	case errors.Is(err, issue.ErrAlreadyRated):
		return maxapi.CallbackAnswer{Notification: "Вы уже оценили этот ремонт."}, nil
	case errors.Is(err, issue.ErrWindowClosed):
		return maxapi.CallbackAnswer{Notification: "Прошло больше 7 дней после ремонта: оценить уже нельзя."}, nil
	case errors.Is(err, issue.ErrNotConfirmed), errors.Is(err, issue.ErrNotParticipant), errors.Is(err, issue.ErrInvalid),
		errors.Is(err, app.ErrNotFound), errors.Is(err, app.ErrForbidden):
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
	case err != nil:
		return maxapi.CallbackAnswer{}, err
	}
	m := screenMsg(fmt.Sprintf("Спасибо! Ваша оценка ремонта: %d из 5. Из оценок жителей складывается рейтинг управляющих компаний района.", stars),
		[]maxapi.Button{maxapi.CallbackButton("Рейтинг УК района", win(scrRating))},
		menuRow(),
	)
	return maxapi.CallbackAnswer{Message: &m, Notification: "Оценка сохранена"}, nil
}

// ratingScreen — рейтинг УК района (ADR-022): жителю по району его дома.
func (h *Handler) ratingScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	r, err := h.svc.Issues.DistrictRating(ctx, u)
	if errors.Is(err, app.ErrForbidden) || errors.Is(err, app.ErrNotFound) {
		return h.homeScreen(ctx, u)
	}
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Рейтинг УК района %s** за %d дней", plain(r.District), issues.MetricsPeriodDays)
	place := 0
	for _, o := range r.Orgs {
		mine := ""
		if o.Org.ID == r.MyOrgID {
			mine = ", ваша УК"
		}
		if !o.Enough {
			fmt.Fprintf(&b, "\n\n%s%s\nМало данных: закрыто заявок %d", plain(o.Org.Name), mine, o.ClosedTotal)
			continue
		}
		place++
		fmt.Fprintf(&b, "\n\n%d. %s%s\n%d из 100", place, plain(o.Org.Name), mine, o.Score)
		if o.Ratings > 0 {
			fmt.Fprintf(&b, ", оценка жителей %s", ratingRU(o.RatingAvg()))
		}
		fmt.Fprintf(&b, ", в срок %d%%", o.ClosedOnTime*100/o.ClosedTotal)
	}
	if len(r.Orgs) == 0 {
		b.WriteString("\n\nВ районе пока нет управляющих компаний с заявками.")
	}
	b.WriteString("\n\nКак считается: 50% заявки, закрытые в срок, 30% оценки ремонта жителями, 20% ремонты, которые жители подтвердили.")
	return screenMsg(b.String(), navRow()), nil
}

// ratingRU — средняя оценка с одним знаком после запятой: «4,6».
func ratingRU(v float64) string {
	return strings.Replace(strconv.FormatFloat(math.Round(v*10)/10, 'f', 1, 64), ".", ",", 1)
}

// changeStatus — нажатие кнопки статуса сотрудником УК. «Выполнено» и «Отклонить» сначала
// спрашивают комментарий для жителей: причина отказа обязательна, комментарий к выполнению нет.
func (h *Handler) changeStatus(ctx context.Context, u user.User, rest string) (maxapi.CallbackAnswer, error) {
	parts := strings.Split(rest, ":")
	if len(parts) < 2 {
		return maxapi.CallbackAnswer{Notification: "Эта кнопка устарела."}, nil
	}
	id, st := parts[0], issue.Status(parts[1])
	skipComment := len(parts) > 2 && parts[2] == "c"
	back := []string{scrIssue, id, scrQueue}
	// Права и переход проверяются до вопроса о комментарии: иначе бот попросит текст, который не примет.
	is, err := h.svc.Issues.Get(ctx, id)
	switch {
	case err != nil:
		return statusError(err)
	case !u.CanManageIssues(is.ResponsibleOrgID()):
		return statusError(app.ErrForbidden)
	case !slices.Contains(issue.NextStatuses(is.Status()), st):
		return statusError(issue.ErrTransition)
	}
	if (st == issue.StatusDone || st == issue.StatusRejected) && !skipComment {
		if err := h.svc.Pending.Set(ctx, u.ID, app.BotPending{Action: pendStatus, Ref: id, Text: string(st), ExpiresAt: h.svc.Now().Add(pendingTTL)}); err != nil {
			return maxapi.CallbackAnswer{}, err
		}
		if st == issue.StatusRejected {
			m := screenMsg("Напишите одним сообщением причину отказа: её увидят жители.", navRow(back...))
			return maxapi.CallbackAnswer{Message: &m}, nil
		}
		m := screenMsg("Напишите одним сообщением, что сделано: жители увидят комментарий и проверят ремонт.",
			[]maxapi.Button{maxapi.CallbackButton("Без комментария", pack(cbStatus, id, string(st), "c"))},
			navRow(back...),
		)
		return maxapi.CallbackAnswer{Message: &m}, nil
	}
	if skipComment {
		h.dropPending(ctx, u)
	}
	if _, err := h.svc.Issues.ChangeStatus(ctx, u, id, st, ""); err != nil {
		return statusError(err)
	}
	m, err := h.issueScreen(ctx, u, id, scrQueue)
	if err != nil {
		return maxapi.CallbackAnswer{}, err
	}
	return maxapi.CallbackAnswer{Message: &m, Notification: "Статус: " + statusWord[st]}, nil
}

// statusError — понятные ответы на ошибки смены статуса.
func statusError(err error) (maxapi.CallbackAnswer, error) {
	switch {
	case errors.Is(err, app.ErrForbidden):
		return maxapi.CallbackAnswer{Notification: "Статус меняет только сотрудник ответственной УК."}, nil
	case errors.Is(err, issue.ErrReasonRequired):
		return maxapi.CallbackAnswer{Notification: "Отказ без причины не отправить: напишите причину для жителей."}, nil
	case errors.Is(err, issue.ErrTransition):
		return maxapi.CallbackAnswer{Notification: "Такой переход статуса невозможен: заявку уже изменили."}, nil
	case errors.Is(err, app.ErrNotFound):
		return maxapi.CallbackAnswer{Notification: "Заявка не найдена."}, nil
	}
	return maxapi.CallbackAnswer{}, err
}

// durationRU — «2 ч 15 мин» или «40 мин» для медианы первого ответа.
func durationRU(d interface{ Minutes() float64 }) string {
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf("%d мин", mins)
	}
	if mins%60 == 0 {
		return fmt.Sprintf("%d ч", mins/60)
	}
	return fmt.Sprintf("%d ч %d мин", mins/60, mins%60)
}

// alertsScreen — текущие плановые работы и отключения в доме жителя (GEN_V4, ADR-024).
func (h *Handler) alertsScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	if u.HouseID == "" || h.svc.Office == nil {
		return screenMsg("В вашем доме сейчас нет плановых отключений и работ.", navRow()), nil
	}
	list, err := h.svc.Office.ActiveMaintenance(ctx, u.HouseID)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	if len(list) == 0 {
		return screenMsg("**Плановые работы.**\nВ вашем доме сейчас нет активных плановых работ и отключений.", navRow()), nil
	}
	var b strings.Builder
	b.WriteString("**Плановые работы в доме:**")
	for i, m := range list {
		fmt.Fprintf(&b, "\n\n%d. **%s**\nДо %s", i+1, plain(m.Title), dayMonth(m.EndsAt))
		if m.Description != "" {
			fmt.Fprintf(&b, "\n%s", plain(m.Description))
		}
	}
	return screenMsg(b.String(),
		[]maxapi.Button{maxapi.OpenAppButton("Открыть в приложении", h.botName, "")},
		navRow(),
	), nil
}

// visitScreen — часы личного приёма граждан в УК и записи жителя (GEN_V4, ADR-024).
func (h *Handler) visitScreen(ctx context.Context, u user.User) (maxapi.NewMessage, error) {
	var b strings.Builder
	b.WriteString("**Личный приём в УК** (по ПП РФ № 416):\n")
	for _, s := range office.Specialists() {
		fmt.Fprintf(&b, "\n• **%s** — %s\n  %s", plain(s.Title), plain(s.Schedule), plain(s.Description))
	}
	if h.svc.Office != nil {
		if mine, err := h.svc.Office.MyAppointments(ctx, u); err == nil && len(mine) > 0 {
			b.WriteString("\n\n**Ваши записи на приём:**")
			for _, a := range mine {
				if a.Status == "cancelled" {
					continue
				}
				specTitle := a.Specialist
				if sp, ok := office.LookupSpecialist(a.Specialist); ok {
					specTitle = sp.Title
				}
				fmt.Fprintf(&b, "\n• %s (%s): %s", plain(specTitle), dayMonth(a.SlotAt), plain(a.Topic))
			}
		}
	}
	return screenMsg(b.String(),
		[]maxapi.Button{maxapi.OpenAppButton("Записаться на приём в приложении", h.botName, "")},
		navRow(),
	), nil
}
