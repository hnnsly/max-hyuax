package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/user"
)

type Config struct {
	BotToken       string
	SessionSecret  string
	SessionTTL     time.Duration
	DemoEnabled    bool   // POST /auth/demo; включается только на демо-стенде
	ConsentVersion string // с этой версией согласия восстанавливается удалённый демо-пользователь
	Now            func() time.Time
}

type Service struct {
	store app.Store
	cfg   Config
}

func NewService(store app.Store, cfg Config) *Service {
	return &Service{store: store, cfg: cfg}
}

type Session struct {
	Token      string
	ExpiresAt  time.Time
	User       user.User
	StartParam string // параметр запуска мини-приложения, если он был в initData
}

// demoKeys — демо-роли для проверяющих; пользователи создаются миграцией с демо-данными.
// Имя, дом и синтетический телефон нужны, чтобы вернуть демо-пользователя, если проверяющий
// удалил аккаунт.
var demoKeys = map[string]struct{ key, name, houseID, phone string }{
	"resident":    {"resident_demo_1", "Анна", "h-17k2", "+79990000001"},
	"resident_2":  {"resident_demo_2", "Сергей", "h-17k2", ""},
	"uk_operator": {"uk_operator_demo", "Оператор УК", "", ""},
	"district":    {"district_demo", "Управа района Зябликово", "", ""},
}

// LoginMax проверяет initData и выдаёт сессию; новый пользователь MAX становится жителем.
func (s *Service) LoginMax(ctx context.Context, initData string) (Session, error) {
	d, err := ParseInitData(initData, s.cfg.BotToken, s.cfg.Now())
	if err != nil {
		return Session{}, fmt.Errorf("%w: %w", app.ErrUnauthorized, err)
	}
	u, err := s.EnsureMaxUser(ctx, d.User.ID, d.User.FirstName)
	if err != nil {
		return Session{}, err
	}
	sess := s.issue(u)
	sess.StartParam = d.StartParam
	return sess, nil
}

// EnsureMaxUser находит пользователя MAX или создаёт жителя. Нужен и мини-приложению, и боту:
// бот знает пользователя по user_id из события.
func (s *Service) EnsureMaxUser(ctx context.Context, maxUserID int64, firstName string) (user.User, error) {
	users := s.store.Users()
	u, err := users.ByMaxID(ctx, maxUserID)
	// Удалённый аккаунт теряет связь с MAX, поэтому вернувшийся житель попадает сюда как новый.
	if errors.Is(err, app.ErrNotFound) {
		return users.Create(ctx, user.User{MaxUserID: maxUserID, FirstName: firstName, Role: user.RoleResident})
	}
	return u, err
}

// LoginDemo входит тестовым пользователем роли без клиента MAX.
func (s *Service) LoginDemo(ctx context.Context, role string) (Session, error) {
	if !s.cfg.DemoEnabled {
		return Session{}, app.ErrForbidden
	}
	demo, ok := demoKeys[role]
	if !ok {
		return Session{}, fmt.Errorf("%w: unknown demo role %q", app.ErrInvalidInput, role)
	}
	u, err := s.store.Users().ByDemoKey(ctx, demo.key)
	if err != nil {
		return Session{}, err
	}
	changed := false
	if u.Deleted() {
		// Проверяющий удалил демо-аккаунт: возвращаем его в исходное состояние из демо-данных.
		u.DeletedAt, u.FirstName, u.HouseID = time.Time{}, demo.name, demo.houseID
		u.AcceptConsent(s.cfg.ConsentVersion, s.cfg.Now())
		changed = true
	}
	if u.Phone != demo.phone {
		// Синтетический телефон демо-жителя возвращается на каждом входе: без MAX его не оставить
		// заново, а без него у УК в демо пропадёт блок «Контакты жителей».
		u.SharePhone(demo.phone)
		changed = true
	}
	if changed {
		if err := s.store.Users().Save(ctx, u); err != nil {
			return Session{}, err
		}
	}
	return s.issue(u), nil
}

// Authenticate проверяет токен сессии и возвращает актуального пользователя из хранилища.
func (s *Service) Authenticate(ctx context.Context, token string) (user.User, error) {
	id, err := verifyToken(s.cfg.SessionSecret, token, s.cfg.Now())
	if err != nil {
		return user.User{}, app.ErrUnauthorized
	}
	u, err := s.store.Users().Get(ctx, id)
	if errors.Is(err, app.ErrNotFound) || (err == nil && u.Deleted()) {
		return user.User{}, app.ErrUnauthorized
	}
	return u, err
}

// AcceptConsent фиксирует согласие на обработку ПДн указанной версии документа.
func (s *Service) AcceptConsent(ctx context.Context, u user.User, version string) (user.User, error) {
	if version == "" {
		return u, fmt.Errorf("%w: consent version is required", app.ErrInvalidInput)
	}
	u.AcceptConsent(version, s.cfg.Now())
	return u, s.store.Users().Save(ctx, u)
}

// SetHouse привязывает жителя к дому, чтобы главный экран показывал его заявки.
func (s *Service) SetHouse(ctx context.Context, u user.User, houseID string) (user.User, error) {
	if _, err := s.store.Houses().Get(ctx, houseID); err != nil {
		return u, err
	}
	u.HouseID = houseID
	return u, s.store.Users().Save(ctx, u)
}

// SharePhone сохраняет телефон для мастера: номер из WebApp.requestContact с подписью MAX.
// Демо-пользователи без аккаунта MAX оставить телефон не могут.
func (s *Service) SharePhone(ctx context.Context, u user.User, c Contact) (user.User, error) {
	if u.MaxUserID == 0 {
		return u, fmt.Errorf("%w: phone can be shared only from MAX", app.ErrForbidden)
	}
	phone, err := VerifyContact(c, u.MaxUserID, s.cfg.BotToken, s.cfg.Now())
	if err != nil {
		return u, fmt.Errorf("%w: %w", app.ErrInvalidInput, err)
	}
	u.SharePhone(phone)
	return u, s.store.Users().Save(ctx, u)
}

// HidePhone стирает телефон: УК больше не видит его ни по одной заявке.
func (s *Service) HidePhone(ctx context.Context, u user.User) (user.User, error) {
	u.HidePhone()
	return u, s.store.Users().Save(ctx, u)
}

// DeleteAccount обезличивает аккаунт; заявки остаются, но без имени и телефона автора.
func (s *Service) DeleteAccount(ctx context.Context, u user.User) error {
	u.Delete(s.cfg.Now())
	return s.store.Users().Save(ctx, u)
}

func (s *Service) issue(u user.User) Session {
	exp := s.cfg.Now().Add(s.cfg.SessionTTL)
	return Session{Token: signToken(s.cfg.SessionSecret, u.ID, exp), ExpiresAt: exp, User: u}
}
