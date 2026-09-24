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
	Now            func() time.Time
	NewID          func() string
	ConsentVersion string // житель прикладывает фото только с согласием на обработку данных
}

// decodeSlots — сколько снимков декодируется одновременно (HTTP и бот вместе): до 128 МБ на каждый.
const decodeSlots = 2

type Service struct {
	store  app.Store
	files  app.FileStore
	cfg    Config
	decode chan struct{}
}

func NewService(store app.Store, files app.FileStore, cfg Config) *Service {
	return &Service{store: store, files: files, cfg: cfg, decode: make(chan struct{}, decodeSlots)}
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

// Add прикладывает пачку фото целиком или не прикладывает ничего: сначала все файлы проверяются
// и перекодируются, затем записываются, затем метаданные вносятся одной транзакцией
// с блокировкой заявки, где лимит пересчитывается. Сбой на любом шаге убирает записанные файлы.
func (s *Service) Add(ctx context.Context, u user.User, issueID string, raws ...[]byte) ([]app.Photo, error) {
	if len(raws) == 0 {
		return nil, app.ErrInvalidInput
	}
	if u.Role == user.RoleResident && !u.HasConsent(s.cfg.ConsentVersion) {
		return nil, app.ErrConsentRequired
	}
	is, err := s.issueFor(ctx, u, issueID)
	if err != nil {
		return nil, err
	}
	if is.Status().Closed() {
		return nil, issue.ErrClosed
	}
	// Быстрая проверка до тяжёлого декодирования; окончательная — в транзакции.
	existing, err := s.store.Photos().ListByIssue(ctx, is.ID())
	if err != nil {
		return nil, err
	}
	if len(existing)+len(raws) > MaxPerIssue {
		return nil, ErrTooMany
	}

	added := make([]app.Photo, 0, len(raws))
	data := make([][]byte, 0, len(raws))
	for _, raw := range raws {
		b, w, h, err := s.normalize(ctx, raw)
		if err != nil {
			return nil, err
		}
		added = append(added, app.Photo{
			ID: s.cfg.NewID(), IssueID: is.ID(), UploadedBy: u.ID,
			Width: w, Height: h, SizeBytes: len(b), CreatedAt: s.cfg.Now(),
		})
		data = append(data, b)
	}
	for i, p := range added {
		if err := s.files.Put(ctx, p.ID, data[i]); err != nil {
			s.dropFiles(ctx, added[:i])
			return nil, err
		}
	}
	err = s.store.InTx(ctx, func(tx app.Store) error {
		if _, err := tx.Issues().GetForUpdate(ctx, is.ID()); err != nil {
			return err
		}
		existing, err := tx.Photos().ListByIssue(ctx, is.ID())
		if err != nil {
			return err
		}
		if len(existing)+len(added) > MaxPerIssue {
			return ErrTooMany
		}
		for _, p := range added {
			if err := tx.Photos().Add(ctx, p); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.dropFiles(ctx, added)
		return nil, err
	}
	return added, nil
}

// normalize ждёт свободный слот: одновременно декодируется не больше decodeSlots снимков.
func (s *Service) normalize(ctx context.Context, raw []byte) ([]byte, int, int, error) {
	select {
	case s.decode <- struct{}{}:
	case <-ctx.Done():
		return nil, 0, 0, ctx.Err()
	}
	defer func() { <-s.decode }()
	return normalize(raw)
}

// dropFiles убирает файлы, для которых не появились метаданные. Ошибку удаления не возвращаем:
// главное — исходная ошибка, а лишний файл без записи в базе никому не виден.
func (s *Service) dropFiles(ctx context.Context, list []app.Photo) {
	for _, p := range list {
		_ = s.files.Delete(context.WithoutCancel(ctx), p.ID)
	}
}

// Remove убирает фото; убрать может только тот, кто его приложил.
func (s *Service) Remove(ctx context.Context, u user.User, photoID string) error {
	p, err := s.store.Photos().Get(ctx, photoID)
	if err != nil {
		return err
	}
	if p.UploadedBy != u.ID {
		return app.ErrForbidden
	}
	return s.remove(ctx, p)
}

// ForgetUser удаляет все фото пользователя: часть удаления аккаунта, на снимках бывают люди и двери квартир.
func (s *Service) ForgetUser(ctx context.Context, userID int64) error {
	list, err := s.store.Photos().ListByUploader(ctx, userID)
	if err != nil {
		return err
	}
	for _, p := range list {
		if err := s.remove(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// remove: сначала запись в базе, потом файл — фото исчезает из заявки, даже если файл удалить не вышло.
func (s *Service) remove(ctx context.Context, p app.Photo) error {
	if err := s.store.Photos().Delete(ctx, p.ID); err != nil {
		return err
	}
	return s.files.Delete(ctx, p.ID)
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
