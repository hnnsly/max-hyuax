// Пакет app — слой приложения: порты (интерфейсы хранилища) и общие ошибки сценариев.
// Сами сценарии живут во вложенных пакетах: issues, auth.
package app

import (
	"context"
	"errors"
	"time"

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
	SampleData   bool // среди заявок УК есть синтетические (пример данных)
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
	// Upsert создаёт или обновляет дом по id и добавляет недостающие подъезды и объекты с QR-кодами
	// (лифт и свет в подъезде, кровля, мусоропровод). created — дома раньше не было.
	Upsert(ctx context.Context, h house.House) (created bool, err error)
}

// Geocoder — геокодер на открытых данных (ADR-016). Не нашёл — ok = false без ошибки.
type Geocoder interface {
	// Geocode — координаты дома по адресу реестра («Ореховый бульвар, 15»).
	Geocode(ctx context.Context, address string) (lat, lon float64, ok bool, err error)
	// Reverse — короткий адрес точки: «улица, дом».
	Reverse(ctx context.Context, lat, lon float64) (address string, ok bool, err error)
}

type UserRepo interface {
	Get(ctx context.Context, id int64) (user.User, error)
	ByMaxID(ctx context.Context, maxUserID int64) (user.User, error)
	ByDemoKey(ctx context.Context, key string) (user.User, error)
	Create(ctx context.Context, u user.User) (user.User, error)
	Save(ctx context.Context, u user.User) error
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
)

// Notification — намерение уведомить участника. Текст собирается при отправке
// из актуального состояния заявки, поэтому в очереди хранится только адресат.
type Notification struct {
	Kind    NotificationKind
	IssueID string
	UserID  int64
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

// Store — доступ к репозиториям; InTx выполняет fn в одной транзакции.
type Store interface {
	Issues() IssueRepo
	Houses() HouseRepo
	Users() UserRepo
	Outbox() OutboxRepo
	Photos() PhotoRepo
	InTx(ctx context.Context, fn func(tx Store) error) error
}
