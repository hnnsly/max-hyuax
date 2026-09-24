// Пакет photos — фото к заявке: проверка и перекодирование, права участника и УК, хранение.
package photos

import (
	"context"
	"errors"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
)

// MaxPerIssue — сколько фото можно приложить к одной заявке.
const MaxPerIssue = 6

var ErrTooMany = errors.New("photos: too many photos for the issue")

type Config struct {
	Now   func() time.Time
	NewID func() string
}

type Service struct {
	store app.Store
	files app.FileStore
	cfg   Config
}

func NewService(store app.Store, files app.FileStore, cfg Config) *Service {
	return &Service{store: store, files: files, cfg: cfg}
}

// canSee: фото видят участники заявки и сотрудники ответственной УК. Соседи-неучастники — нет:
// на снимке могут оказаться люди и двери квартир.
func canSee(u user.User, is *issue.Issue) bool {
	return is.HasParticipant(u.ID) || (u.Role == user.RoleOperator && u.OrganizationID != "" && u.OrganizationID == is.ResponsibleOrgID())
}

func (s *Service) issueFor(ctx context.Context, u user.User, issueID string) (*issue.Issue, error) {
	is, err := s.store.Issues().Get(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if !canSee(u, is) {
		return nil, app.ErrForbidden
	}
	return is, nil
}

// Add проверяет и перекодирует фото, записывает файл, затем метаданные.
// Порядок важен: при сбое записи файла в заявке не появится фото без файла.
func (s *Service) Add(ctx context.Context, u user.User, issueID string, raw []byte) (app.Photo, error) {
	is, err := s.issueFor(ctx, u, issueID)
	if err != nil {
		return app.Photo{}, err
	}
	if is.Status().Closed() {
		return app.Photo{}, issue.ErrClosed
	}
	existing, err := s.store.Photos().ListByIssue(ctx, is.ID())
	if err != nil {
		return app.Photo{}, err
	}
	if len(existing) >= MaxPerIssue {
		return app.Photo{}, ErrTooMany
	}
	data, w, h, err := normalize(raw)
	if err != nil {
		return app.Photo{}, err
	}
	p := app.Photo{
		ID: s.cfg.NewID(), IssueID: is.ID(), UploadedBy: u.ID,
		Width: w, Height: h, SizeBytes: len(data), CreatedAt: s.cfg.Now(),
	}
	if err := s.files.Put(ctx, p.ID, data); err != nil {
		return app.Photo{}, err
	}
	if err := s.store.Photos().Add(ctx, p); err != nil {
		return app.Photo{}, err
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, u user.User, issueID string) ([]app.Photo, error) {
	is, err := s.issueFor(ctx, u, issueID)
	if err != nil {
		return nil, err
	}
	return s.store.Photos().ListByIssue(ctx, is.ID())
}

// Open возвращает фото и его JPEG, если пользователю видна заявка.
func (s *Service) Open(ctx context.Context, u user.User, photoID string) (app.Photo, []byte, error) {
	p, err := s.store.Photos().Get(ctx, photoID)
	if err != nil {
		return app.Photo{}, nil, err
	}
	if _, err := s.issueFor(ctx, u, p.IssueID); err != nil {
		return app.Photo{}, nil, err
	}
	data, err := s.files.Get(ctx, p.ID)
	if err != nil {
		return app.Photo{}, nil, err
	}
	return p, data, nil
}
