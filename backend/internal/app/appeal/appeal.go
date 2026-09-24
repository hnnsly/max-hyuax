// Пакет appeal — черновик обращения в жилинспекцию по просроченной заявке.
// Мини-приложение получает короткоживущую подписанную ссылку: в MAX файл скачивается
// через WebApp.downloadFile, а он не передаёт заголовок с токеном сессии.
package appeal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"dommax/internal/app"
	"dommax/internal/domain/house"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/rules"
	"dommax/internal/domain/user"
)

// ErrNotOverdue — срок ответа ещё не истёк, обращаться в жилинспекцию рано.
var ErrNotOverdue = errors.New("appeal: issue is not overdue")

type Config struct {
	Secret []byte        // ключ подписи ссылок; из него выводится отдельный ключ
	TTL    time.Duration // сколько действует ссылка
	Now    func() time.Time
}

type Service struct {
	store app.Store
	key   []byte
	cfg   Config
}

func NewService(store app.Store, cfg Config) *Service {
	// Отдельный ключ: подпись ссылки нельзя выдать за токен сессии и наоборот.
	m := hmac.New(sha256.New, cfg.Secret)
	m.Write([]byte("appeal-link"))
	return &Service{store: store, key: m.Sum(nil), cfg: cfg}
}

type Link struct {
	Token     string
	ExpiresAt time.Time
}

// Document — всё, что нужно для текста обращения. Имён соседей и автора в нём нет.
type Document struct {
	Number       int64
	Address      string
	Place        string // подпись объекта, если заявка по объекту с QR
	Category     string
	Title        string
	Description  string
	Organization string
	CreatedAt    time.Time
	Deadline     time.Time
	Participants int
	Basis        string
	Events       []issue.Event
	GeneratedAt  time.Time
}

// Prepare выдаёт ссылку на обращение: только участнику открытой заявки с истёкшим сроком.
func (s *Service) Prepare(ctx context.Context, u user.User, issueID string) (Link, error) {
	is, err := s.store.Issues().Get(ctx, issueID)
	if err != nil {
		return Link{}, err
	}
	now := s.cfg.Now()
	if err := allowed(is, u.ID, now); err != nil {
		return Link{}, err
	}
	exp := now.Add(s.cfg.TTL)
	payload := fmt.Sprintf("%s|%d|%d", is.ID(), u.ID, exp.Unix())
	return Link{Token: enc(payload) + "." + enc(string(s.sign(payload))), ExpiresAt: exp}, nil
}

// allowed — обращение готовит только участник открытой заявки с истёкшим сроком.
func allowed(is *issue.Issue, userID int64, now time.Time) error {
	switch {
	case !is.HasParticipant(userID):
		return app.ErrForbidden
	case is.Status().Closed():
		return issue.ErrClosed
	case !is.IsOverdue(now):
		return ErrNotOverdue
	}
	return nil
}

// Document проверяет подпись и срок ссылки и собирает данные обращения. Условия выдачи
// проверяются заново: за время жизни ссылки заявку могли закрыть, а аккаунт удалить.
func (s *Service) Document(ctx context.Context, token string) (Document, error) {
	issueID, userID, err := s.verify(token)
	if err != nil {
		return Document{}, err
	}
	if u, err := s.store.Users().Get(ctx, userID); err != nil || u.Deleted() {
		return Document{}, fmt.Errorf("%w: appeal link owner is gone", app.ErrForbidden)
	}
	is, err := s.store.Issues().Get(ctx, issueID)
	if err != nil {
		return Document{}, err
	}
	if err := allowed(is, userID, s.cfg.Now()); err != nil {
		return Document{}, err
	}
	houses := s.store.Houses()
	h, err := houses.Get(ctx, is.HouseID())
	if err != nil {
		return Document{}, err
	}
	org, err := houses.Organization(ctx, is.ResponsibleOrgID())
	if err != nil {
		return Document{}, err
	}
	events, err := s.store.Issues().Events(ctx, is.ID())
	if err != nil {
		return Document{}, err
	}
	d := Document{
		Number: is.Number(), Address: h.Address, Title: is.Title(), Description: is.Description(),
		Organization: org.Name, CreatedAt: is.CreatedAt(), Deadline: is.Deadline(),
		Participants: is.ParticipantCount(), Events: events, GeneratedAt: s.cfg.Now(),
	}
	if r, err := rules.Lookup(is.Category()); err == nil {
		d.Category, d.Basis = r.Title, r.Basis
	}
	if is.ObjectID() != "" {
		objects, err := houses.Objects(ctx, is.HouseID())
		if err != nil {
			return Document{}, err
		}
		if i := slices.IndexFunc(objects, func(o house.AssetObject) bool { return o.ID == is.ObjectID() }); i >= 0 {
			d.Place = objects[i].Label
		}
	}
	return d, nil
}

func (s *Service) sign(payload string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// verify возвращает id заявки и пользователя из действующей ссылки; любая ошибка — ErrForbidden.
func (s *Service) verify(token string) (string, int64, error) {
	bad := fmt.Errorf("%w: invalid or expired appeal link", app.ErrForbidden)
	rawPayload, rawSig, ok := strings.Cut(token, ".")
	if !ok {
		return "", 0, bad
	}
	payload, err1 := dec(rawPayload)
	sig, err2 := dec(rawSig)
	if err1 != nil || err2 != nil || !hmac.Equal([]byte(sig), s.sign(payload)) {
		return "", 0, bad
	}
	parts := strings.Split(payload, "|")
	if len(parts) != 3 {
		return "", 0, bad
	}
	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, bad
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || !s.cfg.Now().Before(time.Unix(exp, 0)) {
		return "", 0, bad
	}
	return parts[0], userID, nil
}

func enc(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func dec(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	return string(b), err
}
