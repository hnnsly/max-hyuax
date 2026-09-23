// Пакет httpapi — HTTP API мини-приложения на Fiber v3 и приём webhook MAX.
// JSON-DTO живут только здесь; сценарии вызываются из слоя app.
package httpapi

import (
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app"
	"dommax/internal/app/auth"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
	"dommax/internal/transport/bot"
)

type Deps struct {
	Auth           *auth.Service
	Issues         *issues.Service
	Houses         *houses.Service
	Webhook        *bot.Webhook // nil — webhook выключен (BOT_MODE не webhook)
	Ping           func(context.Context) error
	ConsentVersion string
	Now            func() time.Time
	Log            *slog.Logger
}

type handlers struct{ Deps }

func New(d Deps) *fiber.App {
	h := &handlers{d}
	app := fiber.New(fiber.Config{
		AppName:      "dom-max api",
		ErrorHandler: h.onError,
		JSONEncoder:  func(v any) ([]byte, error) { return json.Marshal(v) },
		JSONDecoder:  func(data []byte, v any) error { return json.Unmarshal(data, v) },
	})

	api := app.Group("/api/v1")
	api.Get("/health", h.health)
	api.Post("/auth/max", h.loginMax)
	api.Post("/auth/demo", h.loginDemo)
	api.Get("/categories", h.categories)

	// Остальное — только с сессией: middleware указан у каждого маршрута явно.
	api.Get("/me", h.auth, h.me)
	api.Post("/me/consent", h.auth, h.acceptConsent)
	api.Post("/me/house", h.auth, h.setHouse)
	api.Delete("/me", h.auth, h.deleteAccount)

	api.Get("/houses", h.auth, h.searchHouses)
	api.Get("/houses/nearest", h.auth, h.nearestHouses)
	api.Get("/houses/:id", h.auth, h.getHouse)
	api.Get("/houses/:id/issues", h.auth, h.houseIssues)
	api.Get("/objects/:code", h.auth, h.objectByCode)

	api.Get("/issues/similar", h.auth, h.similarIssues)
	api.Post("/issues", h.auth, h.reportIssue)
	api.Get("/issues/:id", h.auth, h.getIssue)
	api.Post("/issues/:id/join", h.auth, h.joinIssue)
	api.Post("/issues/:id/status", h.auth, h.changeStatus)

	api.Get("/uk/issues", h.auth, h.ukQueue)

	app.Post("/webhook/max", h.webhook)
	return app
}

const userKey = "user"

// auth проверяет Authorization: Bearer <token> и кладёт пользователя в контекст запроса.
func (h *handlers) auth(c fiber.Ctx) error {
	token, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	if !ok || token == "" {
		return app.ErrUnauthorized
	}
	u, err := h.Auth.Authenticate(c.Context(), token)
	if err != nil {
		return err
	}
	fiber.Locals(c, userKey, u)
	return c.Next()
}

func currentUser(c fiber.Ctx) user.User { return fiber.Locals[user.User](c, userKey) }

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// onError переводит ошибки сценариев в HTTP-коды и понятные жителю сообщения.
func (h *handlers) onError(c fiber.Ctx, err error) error {
	status, code, msg := classify(err)
	if status >= 500 {
		h.Log.ErrorContext(c.Context(), "request failed", "method", c.Method(), "path", c.Route().Path, "err", err)
	}
	return c.Status(status).JSON(map[string]apiError{"error": {Code: code, Message: msg}})
}

func classify(err error) (int, string, string) {
	switch {
	case errors.Is(err, app.ErrUnauthorized):
		return fiber.StatusUnauthorized, "unauthorized", "Нужно войти заново"
	case errors.Is(err, app.ErrConsentRequired):
		return fiber.StatusForbidden, "consent_required", "Нужно согласие на обработку персональных данных"
	case errors.Is(err, app.ErrForbidden):
		return fiber.StatusForbidden, "forbidden", "Недостаточно прав"
	case errors.Is(err, app.ErrNotFound):
		return fiber.StatusNotFound, "not_found", "Не найдено"
	case errors.Is(err, issue.ErrAlreadyJoined):
		return fiber.StatusConflict, "already_joined", "Вы уже присоединились к этой заявке"
	case errors.Is(err, issue.ErrClosed):
		return fiber.StatusConflict, "issue_closed", "Заявка уже закрыта"
	case errors.Is(err, issue.ErrTransition):
		return fiber.StatusConflict, "invalid_transition", "Такой переход статуса невозможен"
	case errors.Is(err, issue.ErrReasonRequired):
		return fiber.StatusUnprocessableEntity, "reason_required", "Укажите причину отказа"
	case errors.Is(err, app.ErrInvalidInput), errors.Is(err, issue.ErrInvalid):
		return fiber.StatusUnprocessableEntity, "invalid_input", "Проверьте введённые данные"
	}
	if fe, ok := errors.AsType[*fiber.Error](err); ok {
		return fe.Code, "http_error", fe.Message
	}
	return fiber.StatusInternalServerError, "internal", "Что-то пошло не так. Попробуйте позже"
}

// bind читает JSON-тело; ошибка разбора — это ошибка ввода, а не сбой сервера.
func bind(c fiber.Ctx, v any) error {
	if err := c.Bind().Body(v); err != nil {
		return errors.Join(app.ErrInvalidInput, err)
	}
	return nil
}

func (h *handlers) health(c fiber.Ctx) error {
	if err := h.Ping(c.Context()); err != nil {
		h.Log.ErrorContext(c.Context(), "health: database unavailable", "err", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(map[string]string{"status": "degraded", "db": "down"})
	}
	return c.JSON(map[string]string{"status": "ok", "db": "ok"})
}

// webhook отвечает MAX сразу: 401 при неверном секрете, 400 при битом теле, иначе 200.
func (h *handlers) webhook(c fiber.Ctx) error {
	if h.Webhook == nil {
		return fiber.ErrNotFound
	}
	err := h.Webhook.Accept(c.Context(), c.Get("X-Max-Bot-Api-Secret"), c.Body())
	switch {
	case errors.Is(err, bot.ErrBadSecret):
		return c.SendStatus(fiber.StatusUnauthorized)
	case errors.Is(err, bot.ErrBadUpdate):
		return c.SendStatus(fiber.StatusBadRequest)
	case err != nil:
		return err
	}
	return c.SendStatus(fiber.StatusOK)
}
