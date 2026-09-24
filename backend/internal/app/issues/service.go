// Пакет issues — сценарии работы с заявками: сообщить, присоединиться, сменить статус,
// найти похожие, очередь УК.
package issues

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dommax/internal/app"
	"dommax/internal/app/cards"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
)

// Moscow — часовой пояс пилота: сроки считаются по московским рабочим дням.
var Moscow = time.FixedZone("MSK", 3*60*60)

const (
	similarWindow = 14 * 24 * time.Hour
	listLimit     = 50
	queueLimit    = 200
)

type Config struct {
	Now            func() time.Time
	NewID          func() string
	ConsentVersion string // текущая версия согласия на обработку ПДн
}

type Service struct {
	store app.Store
	cfg   Config
}

func NewService(store app.Store, cfg Config) *Service {
	return &Service{store: store, cfg: cfg}
}

type ReportInput struct {
	HouseID     string
	ObjectID    string // пусто, если проблема не привязана к объекту с QR
	Category    string // пусто — берётся из объекта
	Title       string // пусто — название категории; место объекта показывается отдельно
	Description string
}

// Report создаёт заявку: ответственный — УК дома, срок — из справочника правил.
func (s *Service) Report(ctx context.Context, u user.User, in ReportInput) (*issue.Issue, error) {
	if !u.HasConsent(s.cfg.ConsentVersion) {
		return nil, app.ErrConsentRequired
	}
	h, err := s.store.Houses().Get(ctx, in.HouseID)
	if err != nil {
		return nil, err
	}
	var obj house.AssetObject
	if in.ObjectID != "" {
		if obj, err = s.object(ctx, h.ID, in.ObjectID); err != nil {
			return nil, err
		}
	}
	rule, err := rules.Lookup(cmp.Or(in.Category, obj.Category))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrInvalidInput, err)
	}
	now := s.cfg.Now().In(Moscow)
	is, err := issue.New(issue.NewParams{
		ID:               s.cfg.NewID(),
		HouseID:          h.ID,
		ObjectID:         obj.ID,
		Category:         rule.Code,
		Title:            cmp.Or(strings.TrimSpace(in.Title), rule.Title),
		Description:      in.Description,
		ResponsibleOrgID: h.OrganizationID,
		ReporterID:       u.ID,
		CreatedAt:        now,
		Deadline:         rule.Deadline(now),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrInvalidInput, err)
	}
	err = s.store.InTx(ctx, func(tx app.Store) error {
		notes := cards.Plan(is.PendingEvents(), is.Participants())
		if err := tx.Issues().Create(ctx, is); err != nil {
			return err
		}
		return tx.Outbox().Enqueue(ctx, notes)
	})
	if err != nil {
		return nil, err
	}
	return is, nil
}

func (s *Service) object(ctx context.Context, houseID, objectID string) (house.AssetObject, error) {
	objs, err := s.store.Houses().Objects(ctx, houseID)
	if err != nil {
		return house.AssetObject{}, err
	}
	for _, o := range objs {
		if o.ID == objectID {
			return o, nil
		}
	}
	return house.AssetObject{}, fmt.Errorf("%w: object %q is not in house %q", app.ErrInvalidInput, objectID, houseID)
}

// Join добавляет жителя к существующей заявке вместо создания дубля.
func (s *Service) Join(ctx context.Context, u user.User, issueID string) (*issue.Issue, error) {
	if !u.HasConsent(s.cfg.ConsentVersion) {
		return nil, app.ErrConsentRequired
	}
	return s.update(ctx, issueID, func(is *issue.Issue) error {
		return is.Join(u.ID, s.cfg.Now())
	})
}

// ChangeStatus доступен только оператору УК, ответственной за заявку.
func (s *Service) ChangeStatus(ctx context.Context, u user.User, issueID string, to issue.Status, comment string) (*issue.Issue, error) {
	return s.update(ctx, issueID, func(is *issue.Issue) error {
		if !u.CanManageIssues(is.ResponsibleOrgID()) {
			return app.ErrForbidden
		}
		return is.ChangeStatus(to, comment, s.cfg.Now())
	})
}

// update загружает заявку с блокировкой, применяет изменение и в одной транзакции
// сохраняет её вместе с уведомлениями участникам (outbox).
func (s *Service) update(ctx context.Context, issueID string, change func(*issue.Issue) error) (*issue.Issue, error) {
	var out *issue.Issue
	err := s.store.InTx(ctx, func(tx app.Store) error {
		is, err := tx.Issues().GetForUpdate(ctx, issueID)
		if err != nil {
			return err
		}
		if err := change(is); err != nil {
			return err
		}
		notes := cards.Plan(is.PendingEvents(), is.Participants())
		if err := tx.Issues().Save(ctx, is); err != nil {
			return err
		}
		out = is
		return tx.Outbox().Enqueue(ctx, notes)
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, issueID string) (*issue.Issue, error) {
	return s.store.Issues().Get(ctx, issueID)
}

func (s *Service) ListByHouse(ctx context.Context, houseID string) ([]*issue.Issue, error) {
	return s.store.Issues().ListByHouse(ctx, houseID, listLimit)
}

// Mine — заявки пользователя: где он автор или присоединился.
func (s *Service) Mine(ctx context.Context, u user.User) ([]*issue.Issue, error) {
	return s.store.Issues().ListByParticipant(ctx, u.ID, listLimit)
}

// Timeline — события заявки для хронологии в карточке.
func (s *Service) Timeline(ctx context.Context, issueID string) ([]issue.Event, error) {
	if _, err := s.store.Issues().Get(ctx, issueID); err != nil {
		return nil, err
	}
	return s.store.Issues().Events(ctx, issueID)
}

// FindSimilar ищет открытые заявки того же дома и категории за последние две недели.
func (s *Service) FindSimilar(ctx context.Context, houseID, category, objectID string) ([]*issue.Issue, error) {
	if houseID == "" || category == "" {
		return nil, fmt.Errorf("%w: house and category are required", app.ErrInvalidInput)
	}
	return s.store.Issues().FindSimilar(ctx, houseID, category, objectID, s.cfg.Now().Add(-similarWindow))
}

// MarkOverdue отмечает просроченные заявки и ставит участникам уведомления; возвращает, сколько отмечено.
// Каждая заявка — в своей транзакции: сбой одной не откатывает остальные и не останавливает пакет.
func (s *Service) MarkOverdue(ctx context.Context) (int, error) {
	now := s.cfg.Now()
	list, err := s.store.Issues().ListOverdueUnmarked(ctx, now, queueLimit)
	if err != nil {
		return 0, err
	}
	marked := 0
	var errs []error
	for _, is := range list {
		_, err := s.update(ctx, is.ID(), func(is *issue.Issue) error { return is.MarkOverdue(now) })
		switch {
		case errors.Is(err, issue.ErrNotOverdue):
			// Заявку успели закрыть или отметить параллельно: ничего не делаем.
		case err != nil:
			errs = append(errs, fmt.Errorf("issue %s: %w", is.ID(), err))
		default:
			marked++
		}
	}
	return marked, errors.Join(errs...)
}

// Queue — заявки УК оператора: сначала открытые по сроку (просроченные первыми), затем закрытые.
func (s *Service) Queue(ctx context.Context, u user.User) ([]*issue.Issue, error) {
	if u.Role != user.RoleOperator || u.OrganizationID == "" {
		return nil, app.ErrForbidden
	}
	return s.store.Issues().Queue(ctx, u.OrganizationID, queueLimit)
}
