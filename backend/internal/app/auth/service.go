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
	BotToken      string
	SessionSecret string
	SessionTTL    time.Duration
	DemoEnabled   bool // POST /auth/demo; включается только на демо-стенде
	Now           func() time.Time
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
var demoKeys = map[string]string{
	"resident":    "resident_demo_1",
	"resident_2":  "resident_demo_2",
	"uk_operator": "uk_operator_demo",
}

// LoginMax проверяет initData и выдаёт сессию; новый пользователь MAX становится жителем.
func (s *Service) LoginMax(ctx context.Context, initData string) (Session, error) {
	d, err := ParseInitData(initData, s.cfg.BotToken, s.cfg.Now())
	if err != nil {
		return Session{}, fmt.Errorf("%w: %w", app.ErrUnauthorized, err)
	}
	users := s.store.Users()
	u, err := users.ByMaxID(ctx, d.User.ID)
	switch {
	case errors.Is(err, app.ErrNotFound):
		u, err = users.Create(ctx, user.User{MaxUserID: d.User.ID, FirstName: d.User.FirstName, Role: user.RoleResident})
	case err == nil && u.Deleted():
		// Житель удалил аккаунт и вернулся: начинаем с чистого листа, согласие нужно заново.
		u.DeletedAt, u.FirstName = time.Time{}, d.User.FirstName
		err = users.Save(ctx, u)
	}
	if err != nil {
		return Session{}, err
	}
	sess := s.issue(u)
	sess.StartParam = d.StartParam
	return sess, nil
}

// LoginDemo входит тестовым пользователем роли без клиента MAX.
func (s *Service) LoginDemo(ctx context.Context, role string) (Session, error) {
	if !s.cfg.DemoEnabled {
		return Session{}, app.ErrForbidden
	}
	key, ok := demoKeys[role]
	if !ok {
		return Session{}, fmt.Errorf("%w: unknown demo role %q", app.ErrInvalidInput, role)
	}
	u, err := s.store.Users().ByDemoKey(ctx, key)
	if err != nil {
		return Session{}, err
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

func (s *Service) issue(u user.User) Session {
	exp := s.cfg.Now().Add(s.cfg.SessionTTL)
	return Session{Token: signToken(s.cfg.SessionSecret, u.ID, exp), ExpiresAt: exp, User: u}
}
