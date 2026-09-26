// Пакет app — слой приложения: порты (интерфейсы хранилища) и общие ошибки сценариев.
// Сами сценарии живут во вложенных пакетах: issues, auth.
package app

import (
	"context"
	"errors"
	"time"

	"dommax/internal/domain/council"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrConsentRequired = errors.New("consent required")
	ErrInvalidInput    = errors.New("invalid input")
)

type IssueRepo interface {
	// Create сохраняет новую заявку и проставляет ей номер из хранилища.
	Create(ctx context.Context, is *issue.Issue) error
	// Save сохраняет состояние, новых участников и накопленные события.
	Save(ctx context.Context, is *issue.Issue) error
	Get(ctx context.Context, id string) (*issue.Issue, error)
	// GetForUpdate блокирует заявку до конца транзакции.
	GetForUpdate(ctx context.Context, id string) (*issue.Issue, error)
	ListByHouse(ctx context.Context, houseID string, limit int) ([]*issue.Issue, error)
	FindSimilar(ctx context.Context, houseID, category, objectID string, since time.Time) ([]*issue.Issue, error)
	Queue(ctx context.Context, orgID string, limit int) ([]*issue.Issue, error)
	// ListByParticipant — заявки, где пользователь автор или присоединился; открытые первыми.
	ListByParticipant(ctx context.Context, userID int64, limit int) ([]*issue.Issue, error)
	// Events — журнал событий заявки по времени.
	Events(ctx context.Context, issueID string) ([]issue.Event, error)
	// ListOverdueUnmarked — открытые заявки с прошедшим сроком, по которым просрочка ещё не отмечена.
	ListOverdueUnmarked(ctx context.Context, now time.Time, limit int) ([]*issue.Issue, error)
	// OverdueInDistrict — открытые заявки домов района с прошедшим сроком, самые давние первыми.
	OverdueInDistrict(ctx context.Context, district string, now time.Time, limit int) ([]*issue.Issue, error)
	// FirstResponses — заявки УК, поданные не раньше since, на которые УК уже ответила.
	FirstResponses(ctx context.Context, orgID string, since time.Time) ([]FirstResponse, error)
	// OrgCounts — счётчики заявок УК: поданные и закрытые не раньше since, открытые на момент now.
	OrgCounts(ctx context.Context, orgID string, since, now time.Time) (OrgCounts, error)
}

// FirstResponse — когда заявку подали и когда УК впервые сменила её статус.
type FirstResponse struct {
	CreatedAt   time.Time
	RespondedAt time.Time
}

// OrgCounts — счётчики для метрик УК.
type OrgCounts struct {
	Issues       int  // подано заявок за период
	Reports      int  // сколько жителей о них сообщили: авторы и присоединившиеся
	ClosedTotal  int  // закрыто за период
	ClosedOnTime int  // из них не позже срока
	Confirmed    int  // из выполненных за период жители подтвердили ремонт
	Reopened     int  // жители вернули в работу за период
	OpenTotal    int  // открыто сейчас
	OverdueOpen  int  // из открытых срок уже прошёл
	RatingSum    int  // сумма оценок жителей 1–5 за ремонты, выполненные за период
	Ratings      int  // сколько оценок
	SampleData   bool // среди заявок УК есть синтетические (пример данных)
}

// RatingAvg — средняя оценка ремонтов; 0, если оценок нет.
func (c OrgCounts) RatingAvg() float64 {
	if c.Ratings == 0 {
		return 0
	}
	return float64(c.RatingSum) / float64(c.Ratings)
}

type HouseRepo interface {
	Search(ctx context.Context, query string) ([]house.House, error)
	Nearest(ctx context.Context, lat, lon float64, limit int) ([]house.House, error)
	Get(ctx context.Context, id string) (house.House, error)
	Organization(ctx context.Context, id string) (house.Organization, error)
	Entrances(ctx context.Context, houseID string) ([]house.Entrance, error)
	Objects(ctx context.Context, houseID string) ([]house.AssetObject, error)
	ObjectByCode(ctx context.Context, code string) (house.AssetObject, error)
	// ByOrganization — дома организации по адресу.
	ByOrganization(ctx context.Context, orgID string) ([]house.House, error)
	// OrganizationsInDistrict — организации, у которых есть дома в районе, по названию.
	OrganizationsInDistrict(ctx context.Context, district string) ([]house.Organization, error)
	// UpsertOrganization создаёт или обновляет организацию (например, районное отделение ГБУ «Жилищник»).
	UpsertOrganization(ctx context.Context, o house.Organization) error
	// Upsert создаёт или обновляет дом по id и добавляет недостающие подъезды и объекты с QR-кодами
	// (лифт и свет в подъезде, кровля, мусоропровод). created — дома раньше не было.
	Upsert(ctx context.Context, h house.House) (created bool, err error)
	// Load — дома УК (или района, если orgID пуст) с координатами и нагрузкой на момент now: для карты.
	Load(ctx context.Context, orgID, district string, now time.Time) ([]HouseLoad, error)
}

// HouseLoad — дом на карте: открытые и просроченные заявки.
type HouseLoad struct {
	House         house.House // заполнены id, адрес и координаты
	Open, Overdue int
	OldestOverdue time.Time // самый ранний прошедший срок; равен now, если просрочек нет
}

// GeoHouse — дом, найденный геокодером OpenStreetMap.
type GeoHouse struct {
	Address  string // «улица, дом»
	District string // название района («Тверской», «Басманный» и т.п.)
	Lat      float64
	Lon      float64
	InMoscow bool // находится ли точка в границах Москвы
}

// Geocoder — геокодер на открытых данных (ADR-016). Не нашёл — ok = false без ошибки.
type Geocoder interface {
	// Geocode — координаты дома по адресу реестра («Ореховый бульвар, 15»).
	Geocode(ctx context.Context, address string) (lat, lon float64, ok bool, err error)
	// Reverse — короткий адрес точки: «улица, дом».
	Reverse(ctx context.Context, lat, lon float64) (address string, ok bool, err error)
	// ReverseHouse определяет точный дом, район и принадлежность к Москве по координатам.
	ReverseHouse(ctx context.Context, lat, lon float64) (GeoHouse, bool, error)
	// SearchHouses ищет дома по текстовому запросу в границах Москвы.
	SearchHouses(ctx context.Context, query string) ([]GeoHouse, error)
}

type UserRepo interface {
	Get(ctx context.Context, id int64) (user.User, error)
	ByMaxID(ctx context.Context, maxUserID int64) (user.User, error)
	ByDemoKey(ctx context.Context, key string) (user.User, error)
	Create(ctx context.Context, u user.User) (user.User, error)
	Save(ctx context.Context, u user.User) error
	// Chairmen — председатели совета дома, без удалённых аккаунтов.
	Chairmen(ctx context.Context, houseID string) ([]user.User, error)
}

type NotificationKind string

const (
	// NotifyCard — отправить или обновить живую карточку заявки у участника.
	NotifyCard NotificationKind = "card"
	// NotifyFinal — отдельное сообщение участнику, когда заявка закрыта.
	NotifyFinal NotificationKind = "final"
	// NotifyOverdue — отдельное сообщение участнику, когда истёк срок ответа.
	NotifyOverdue NotificationKind = "overdue"
	// NotifyReopened — отдельное сообщение соседям, когда житель вернул выполненную заявку в работу.
	NotifyReopened NotificationKind = "reopened"
	// NotifyStatus — отдельное сообщение участникам, когда УК приняла заявку или взяла её в работу:
	// правка живой карточки телефон не подсвечивает, и житель не узнал бы о движении.
	NotifyStatus NotificationKind = "status"
	// NotifyProposal — председателю совета: соседи прислали предложение (ProposalID).
	NotifyProposal NotificationKind = "proposal"
	// NotifyProposalAnswer — автору предложения: председатель ответил.
	NotifyProposalAnswer NotificationKind = "proposal_answer"
)

// Notification — намерение уведомить участника. Текст собирается при отправке
// из актуального состояния заявки, поэтому в очереди хранится только адресат.
type Notification struct {
	Kind       NotificationKind
	IssueID    string
	ProposalID string // для уведомлений совета дома вместо IssueID
	UserID     int64
}

type OutboxItem struct {
	ID int64
	Notification
	Attempts int
}

// OutboxRepo — очередь исходящих сообщений бота и ссылки на отправленные карточки.
type OutboxRepo interface {
	// Enqueue ставит уведомления в очередь; одинаковые ещё не отправленные схлопываются.
	Enqueue(ctx context.Context, notes []Notification) error
	// Claim забирает до n готовых к отправке уведомлений.
	Claim(ctx context.Context, n int) ([]OutboxItem, error)
	Done(ctx context.Context, id int64) error
	// Retry возвращает уведомление в очередь на время at; failed — больше не пытаться.
	Retry(ctx context.Context, id int64, at time.Time, reason string, failed bool) error
	// Release возвращает в очередь уведомления, взятые до перезапуска сервиса.
	Release(ctx context.Context) error
	// CardMID — id сообщения с карточкой заявки у пользователя; ErrNotFound, если ещё не отправляли.
	CardMID(ctx context.Context, issueID string, userID int64) (string, error)
	SaveCardMID(ctx context.Context, issueID string, userID int64, mid string) error
}

// Photo — метаданные фото к заявке; сам файл лежит в FileStore под ключом ID.
type Photo struct {
	ID         string
	IssueID    string
	UploadedBy int64
	Width      int
	Height     int
	SizeBytes  int
	CreatedAt  time.Time
}

type PhotoRepo interface {
	Add(ctx context.Context, p Photo) error
	// ListByIssue — фото заявки в порядке загрузки.
	ListByIssue(ctx context.Context, issueID string) ([]Photo, error)
	// ListByUploader — все фото пользователя: нужны при удалении аккаунта.
	ListByUploader(ctx context.Context, userID int64) ([]Photo, error)
	Get(ctx context.Context, id string) (Photo, error)
	Delete(ctx context.Context, id string) error
}

// FileStore — хранилище файлов вне базы (фото к заявкам).
type FileStore interface {
	Put(ctx context.Context, key string, data []byte) error
	// Get возвращает ErrNotFound, если файла нет.
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete удаляет файл; отсутствие файла не ошибка.
	Delete(ctx context.Context, key string) error
}

// Tally — итоги опроса: голоса по вариантам и вариант пользователя (-1 — не голосовал).
type Tally struct {
	Votes []int
	Mine  int
}

// CouncilRepo — предложения председателю совета и опросы дома (ADR-017).
type CouncilRepo interface {
	AddProposal(ctx context.Context, p council.Proposal) error
	GetProposal(ctx context.Context, id string) (council.Proposal, error)
	// ReplyProposal сохраняет ответ председателя; false — на предложение уже ответили.
	ReplyProposal(ctx context.Context, p council.Proposal) (bool, error)
	// HouseProposals — папка председателя: новые первыми.
	HouseProposals(ctx context.Context, houseID string, limit int) ([]council.Proposal, error)
	AuthorProposals(ctx context.Context, authorID int64, limit int) ([]council.Proposal, error)
	AddPoll(ctx context.Context, p council.Poll) error
	GetPoll(ctx context.Context, id string) (council.Poll, error)
	HousePolls(ctx context.Context, houseID string, limit int) ([]council.Poll, error)
	// Vote сохраняет голос; false — пользователь уже голосовал.
	Vote(ctx context.Context, pollID string, userID int64, option int) (bool, error)
	Tally(ctx context.Context, p council.Poll, userID int64) (Tally, error)
}

// BotPending — действие бота, которое ждёт следующего текстового сообщения пользователя:
// комментарий к «Не починили», текст предложения совету, описание проблемы до выбора места.
type BotPending struct {
	Action    string
	Ref       string // id заявки, категория и т.п., смысл задаёт действие
	Text      string // текст, сохранённый до следующего шага
	ExpiresAt time.Time
}

// BotPendingRepo — одно ожидающее действие на пользователя.
type BotPendingRepo interface {
	// Set заменяет ожидающее действие пользователя.
	Set(ctx context.Context, userID int64, p BotPending) error
	// Take забирает действие и стирает его; ok = false, если действия нет или оно истекло.
	Take(ctx context.Context, userID int64, now time.Time) (p BotPending, ok bool, err error)
}

// Signature — житель поддержал обращение в жилинспекцию по просроченной заявке (ADR-023).
// ФИО и квартира необязательны: без них подпись идёт в счётчик «поддержали».
type Signature struct {
	IssueID   string
	UserID    int64
	FullName  string
	Apartment string
	SignedAt  time.Time
}

// AppealRepo — подписи под коллективным обращением: одна на жителя и заявку.
type AppealRepo interface {
	// Sign ставит или обновляет подпись жителя.
	Sign(ctx context.Context, s Signature) error
	Withdraw(ctx context.Context, issueID string, userID int64) error
	// Signatures — подписи заявки по времени.
	Signatures(ctx context.Context, issueID string) ([]Signature, error)
	// ForgetUser удаляет все подписи пользователя: часть удаления аккаунта.
	ForgetUser(ctx context.Context, userID int64) error
}

// Store — доступ к репозиториям; InTx выполняет fn в одной транзакции.
type Store interface {
	Issues() IssueRepo
	Houses() HouseRepo
	Users() UserRepo
	Outbox() OutboxRepo
	Photos() PhotoRepo
	Council() CouncilRepo
	Pending() BotPendingRepo
	Appeals() AppealRepo
	InTx(ctx context.Context, fn func(tx Store) error) error
}
