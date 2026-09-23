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
}

type HouseRepo interface {
	Search(ctx context.Context, query string) ([]house.House, error)
	Nearest(ctx context.Context, lat, lon float64, limit int) ([]house.House, error)
	Get(ctx context.Context, id string) (house.House, error)
	Organization(ctx context.Context, id string) (house.Organization, error)
	Entrances(ctx context.Context, houseID string) ([]house.Entrance, error)
	Objects(ctx context.Context, houseID string) ([]house.AssetObject, error)
	ObjectByCode(ctx context.Context, code string) (house.AssetObject, error)
}

type UserRepo interface {
	Get(ctx context.Context, id int64) (user.User, error)
	ByMaxID(ctx context.Context, maxUserID int64) (user.User, error)
	ByDemoKey(ctx context.Context, key string) (user.User, error)
	Create(ctx context.Context, u user.User) (user.User, error)
	Save(ctx context.Context, u user.User) error
}

// Store — доступ к репозиториям; InTx выполняет fn в одной транзакции.
type Store interface {
	Issues() IssueRepo
	Houses() HouseRepo
	Users() UserRepo
	InTx(ctx context.Context, fn func(tx Store) error) error
}
