// Пакет office — сценарии плановых работ в доме и записи на личный приём в УК (GEN_V4, ADR-024).
package office

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"dommax/internal/app"
	"dommax/internal/domain/user"
)

const listLimit = 100

type Config struct {
	Now            func() time.Time
	NewID          func() string
	ConsentVersion string
}

type Service struct {
	store app.Store
	cfg   Config
}

func NewService(store app.Store, cfg Config) *Service {
	return &Service{store: store, cfg: cfg}
}

// Specialist — должностное лицо УК, ведущее личный приём граждан (ПП РФ № 416 п. 28).
type Specialist struct {
	Code        string
	Title       string
	Description string
	Schedule    string
}

var specialists = []Specialist{
	{
		Code:        "chief_engineer",
		Title:       "Главный инженер",
		Description: "Инженерные сети, отопление, водоснабжение, лифты и текущий ремонт дома",
		Schedule:    "Вторник и четверг, 10:00 – 18:00",
	},
	{
		Code:        "director",
		Title:       "Руководитель УК",
		Description: "Личный приём граждан по общим и спорным вопросам содержания дома",
		Schedule:    "Среда, 15:00 – 19:00",
	},
	{
		Code:        "accountant",
		Title:       "Бухгалтерия и паспортный стол",
		Description: "Начисления ЖКУ, перерасчёты, сверка показаний и справки",
		Schedule:    "Будни, 09:00 – 18:00",
	},
}

// Specialists возвращает справочник специалистов для записи на приём.
func Specialists() []Specialist {
	return slices.Clone(specialists)
}

// LookupSpecialist находит специалиста по коду.
func LookupSpecialist(code string) (Specialist, bool) {
	i := slices.IndexFunc(specialists, func(s Specialist) bool { return s.Code == code })
	if i < 0 {
		return Specialist{}, false
	}
	return specialists[i], true
}

type CreateMaintenanceInput struct {
	HouseID     string
	Category    string
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
}

// ActiveMaintenance — текущие и предстоящие плановые работы дома.
func (s *Service) ActiveMaintenance(ctx context.Context, houseID string) ([]app.MaintenanceAlert, error) {
	if _, err := s.store.Houses().Get(ctx, houseID); err != nil {
		return nil, err
	}
	return s.store.Office().ActiveMaintenanceByHouse(ctx, houseID, s.cfg.Now())
}

// CreateMaintenance создаёт объявление о плановых работах в доме (только сотрудник ответственной УК).
func (s *Service) CreateMaintenance(ctx context.Context, u user.User, in CreateMaintenanceInput) (app.MaintenanceAlert, error) {
	if u.Role != user.RoleOperator || u.OrganizationID == "" {
		return app.MaintenanceAlert{}, app.ErrForbidden
	}
	h, err := s.store.Houses().Get(ctx, in.HouseID)
	if err != nil {
		return app.MaintenanceAlert{}, err
	}
	if h.OrganizationID != u.OrganizationID {
		return app.MaintenanceAlert{}, app.ErrForbidden
	}
	title := strings.TrimSpace(in.Title)
	desc := strings.TrimSpace(in.Description)
	cat := strings.TrimSpace(in.Category)
	if cat == "" {
		cat = "other"
	}
	if title == "" || utf8.RuneCountInString(title) > 200 || utf8.RuneCountInString(desc) > 1000 {
		return app.MaintenanceAlert{}, fmt.Errorf("%w: invalid maintenance title or description", app.ErrInvalidInput)
	}
	now := s.cfg.Now()
	starts := in.StartsAt
	if starts.IsZero() {
		starts = now
	}
	ends := in.EndsAt
	if ends.IsZero() {
		ends = starts.Add(4 * time.Hour)
	}
	if !ends.After(starts) || !ends.After(now) {
		return app.MaintenanceAlert{}, fmt.Errorf("%w: ends_at must be after starts_at and now", app.ErrInvalidInput)
	}
	m := app.MaintenanceAlert{
		ID:          s.cfg.NewID(),
		HouseID:     h.ID,
		Address:     h.Address,
		Category:    cat,
		Title:       title,
		Description: desc,
		StartsAt:    starts,
		EndsAt:      ends,
		CreatedBy:   u.ID,
		CreatedAt:   now,
	}
	if err := s.store.Office().AddMaintenance(ctx, m); err != nil {
		return app.MaintenanceAlert{}, err
	}
	return m, nil
}

// ListOrgMaintenance — список всех плановых работ домов своей УК.
func (s *Service) ListOrgMaintenance(ctx context.Context, u user.User) ([]app.MaintenanceAlert, error) {
	if u.Role != user.RoleOperator || u.OrganizationID == "" {
		return nil, app.ErrForbidden
	}
	return s.store.Office().ListMaintenanceByOrg(ctx, u.OrganizationID, listLimit)
}

// DeleteMaintenance удаляет/завершает плановые работы.
func (s *Service) DeleteMaintenance(ctx context.Context, u user.User, id string) error {
	if u.Role != user.RoleOperator || u.OrganizationID == "" {
		return app.ErrForbidden
	}
	return s.store.Office().DeleteMaintenance(ctx, id)
}

type BookInput struct {
	Specialist string
	Topic      string
	SlotAt     time.Time
}

// BookAppointment записывает жителя на личный приём к специалисту УК.
func (s *Service) BookAppointment(ctx context.Context, u user.User, in BookInput) (app.Appointment, error) {
	if !u.CanTakePart() || u.HouseID == "" {
		return app.Appointment{}, app.ErrForbidden
	}
	if !u.HasConsent(s.cfg.ConsentVersion) {
		return app.Appointment{}, app.ErrConsentRequired
	}
	if _, ok := LookupSpecialist(in.Specialist); !ok {
		return app.Appointment{}, fmt.Errorf("%w: unknown specialist %q", app.ErrInvalidInput, in.Specialist)
	}
	topic := strings.TrimSpace(in.Topic)
	if utf8.RuneCountInString(topic) < 3 || utf8.RuneCountInString(topic) > 500 {
		return app.Appointment{}, fmt.Errorf("%w: topic must be 3..500 characters", app.ErrInvalidInput)
	}
	h, err := s.store.Houses().Get(ctx, u.HouseID)
	if err != nil {
		return app.Appointment{}, err
	}
	now := s.cfg.Now()
	slot := in.SlotAt
	if slot.IsZero() {
		// По умолчанию следующий день в 14:00
		next := now.Add(24 * time.Hour)
		slot = time.Date(next.Year(), next.Month(), next.Day(), 14, 0, 0, 0, now.Location())
	}
	if !slot.After(now) {
		return app.Appointment{}, fmt.Errorf("%w: slot must be in the future", app.ErrInvalidInput)
	}
	a := app.Appointment{
		ID:         s.cfg.NewID(),
		HouseID:    h.ID,
		Address:    h.Address,
		UserID:     u.ID,
		UserName:   u.FirstName,
		Specialist: in.Specialist,
		Topic:      topic,
		SlotAt:     slot,
		Status:     "booked",
		CreatedAt:  now,
	}
	if err := s.store.Office().AddAppointment(ctx, a); err != nil {
		return app.Appointment{}, err
	}
	return a, nil
}

// MyAppointments — записи текущего жителя на приём в УК.
func (s *Service) MyAppointments(ctx context.Context, u user.User) ([]app.Appointment, error) {
	return s.store.Office().ListUserAppointments(ctx, u.ID, listLimit)
}

// OrgAppointments — график записей жителей для сотрудника УК.
func (s *Service) OrgAppointments(ctx context.Context, u user.User) ([]app.Appointment, error) {
	if u.Role != user.RoleOperator || u.OrganizationID == "" {
		return nil, app.ErrForbidden
	}
	return s.store.Office().ListOrgAppointments(ctx, u.OrganizationID, listLimit)
}

// CancelAppointment отменяет запись на приём (может сам житель или сотрудник ответственной УК).
func (s *Service) CancelAppointment(ctx context.Context, u user.User, id string) (app.Appointment, error) {
	a, err := s.store.Office().GetAppointment(ctx, id)
	if err != nil {
		return app.Appointment{}, err
	}
	canCancel := a.UserID == u.ID
	if !canCancel && u.Role == user.RoleOperator && u.OrganizationID != "" {
		if h, err := s.store.Houses().Get(ctx, a.HouseID); err == nil && h.OrganizationID == u.OrganizationID {
			canCancel = true
		}
	}
	if !canCancel {
		return app.Appointment{}, app.ErrForbidden
	}
	if err := s.store.Office().CancelAppointment(ctx, id); err != nil {
		return app.Appointment{}, err
	}
	a.Status = "cancelled"
	return a, nil
}
