// Пакет httpapi — HTTP API мини-приложения на Fiber v3 и приём webhook MAX.
// JSON-DTO живут только здесь; сценарии вызываются из слоя app.
package httpapi

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app"
	"dommax/internal/app/appeal"
	"dommax/internal/app/auth"
	"dommax/internal/app/council"
	"dommax/internal/app/hints"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/app/office"
	"dommax/internal/app/photos"
	"dommax/internal/domain/issue"
	"dommax/internal/domain/user"
	"dommax/internal/transport/bot"
)

type Deps struct {
	Auth           *auth.Service
	Issues         *issues.Service
	Houses         *houses.Service
	Hints          *hints.Service
	Appeal         *appeal.Service
	Photos         *photos.Service
	Council        *council.Service
	Office         *office.Service
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
		// Три фото по 5 МБ и служебные поля формы; остальные запросы маленькие.
		BodyLimit:   16 << 20,
		JSONEncoder: func(v any) ([]byte, error) { return json.Marshal(v) },
		JSONDecoder: func(data []byte, v any) error { return json.Unmarshal(data, v) },
	})

	api := app.Group("/api/v1")
	api.Get("/health", h.health)
	api.Post("/auth/max", h.loginMax)
	api.Post("/auth/demo", h.loginDemo)
	api.Get("/categories", h.categories)
	api.Post("/classify", h.auth, h.classifyText)

	// Остальное — только с сессией: middleware указан у каждого маршрута явно.
	api.Get("/me", h.auth, h.me)
	api.Post("/me/consent", h.auth, h.acceptConsent)
	api.Post("/me/house", h.auth, h.setHouse)
	api.Post("/me/role", h.auth, h.switchMyRole)
	api.Delete("/me", h.auth, h.deleteAccount)
	api.Post("/me/phone", h.auth, h.sharePhone)
	api.Delete("/me/phone", h.auth, h.hidePhone)
	api.Get("/me/issues", h.auth, h.myIssues)

	api.Get("/houses", h.auth, h.searchHouses)
	api.Get("/houses/nearest", h.auth, h.nearestHouses)
	api.Get("/geo/reverse", h.auth, h.reverseGeocode)
	api.Get("/houses/:id", h.auth, h.getHouse)
	api.Get("/houses/:id/issues", h.auth, h.houseIssues)
	api.Get("/houses/:id/report.pdf", h.houseReportPDF)
	api.Get("/objects/:code", h.auth, h.objectByCode)
	api.Get("/objects/:code/sticker.pdf", h.objectStickerPDF)

	api.Get("/issues/similar", h.auth, h.similarIssues)
	api.Post("/issues", h.auth, h.reportIssue)
	api.Get("/issues/:id", h.auth, h.getIssue)
	api.Get("/issues/:id/timeline", h.auth, h.timeline)
	api.Post("/issues/:id/join", h.auth, h.joinIssue)
	api.Post("/issues/:id/status", h.auth, h.changeStatus)
	api.Post("/issues/:id/confirm", h.auth, h.confirmRepair)
	api.Post("/issues/:id/reopen", h.auth, h.reopenRepair)
	api.Post("/issues/:id/rating", h.auth, h.rateRepair)
	api.Post("/issues/:id/appeal", h.auth, h.prepareAppeal)
	api.Get("/issues/:id/appeal/signatures", h.auth, h.appealSignatures)
	api.Post("/issues/:id/appeal/sign", h.auth, h.signAppeal)
	api.Delete("/issues/:id/appeal/sign", h.auth, h.withdrawAppeal)
	api.Post("/issues/:id/photos", h.auth, h.uploadPhotos)
	api.Get("/issues/:id/photos", h.auth, h.listPhotos)
	api.Get("/photos/:id", h.auth, h.openPhoto)
	api.Delete("/photos/:id", h.auth, h.removePhoto)
	// Без сессии: ссылка подписана и живёт 10 минут, а WebApp.downloadFile не передаёт заголовки.
	api.Get("/appeal/:token", h.appealPDF)

	api.Get("/uk/issues", h.auth, h.ukQueue)
	api.Get("/uk/metrics", h.auth, h.ukMetrics)
	api.Get("/uk/houses", h.auth, h.ukHouses)

	// Совет дома (ADR-017): предложения председателю и опросы без юридической силы.
	api.Post("/proposals", h.auth, h.propose)
	api.Get("/me/proposals", h.auth, h.myProposals)
	api.Get("/council/proposals", h.auth, h.councilFolder)
	api.Post("/council/proposals/:id/reply", h.auth, h.replyProposal)
	api.Post("/council/polls", h.auth, h.createPoll)
	api.Get("/polls", h.auth, h.housePolls)
	api.Post("/polls/:id/vote", h.auth, h.vote)

	api.Get("/district/metrics", h.auth, h.districtMetrics)
	api.Get("/district/overdue", h.auth, h.districtOverdue)
	api.Get("/map/houses", h.auth, h.mapHouses)
	api.Get("/district/rating", h.auth, h.districtRating)

	// Плановые работы дома и запись на личный приём в УК (GEN_V4, ADR-024).
	api.Get("/houses/:id/maintenance", h.auth, h.houseMaintenance)
	api.Get("/uk/maintenance", h.auth, h.ukMaintenance)
	api.Post("/uk/maintenance", h.auth, h.createMaintenance)
	api.Delete("/uk/maintenance/:id", h.auth, h.deleteMaintenance)
	api.Get("/appointments/specialists", h.auth, h.specialists)
	api.Post("/appointments", h.auth, h.bookAppointment)
	api.Get("/me/appointments", h.auth, h.myAppointments)
	api.Delete("/appointments/:id", h.auth, h.cancelAppointment)
	api.Get("/uk/appointments", h.auth, h.ukAppointments)

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
	if status, code, msg, ok := classifyCouncil(err); ok {
		return status, code, msg
	}
	switch {
	case errors.Is(err, app.ErrUnauthorized):
		return fiber.StatusUnauthorized, "unauthorized", "Нужно войти заново"
	case errors.Is(err, app.ErrConsentRequired):
		return fiber.StatusForbidden, "consent_required", "Нужно согласие на обработку персональных данных"
	case errors.Is(err, app.ErrForbidden), errors.Is(err, issue.ErrNotParticipant):
		return fiber.StatusForbidden, "forbidden", "Недостаточно прав"
	case errors.Is(err, app.ErrNotFound):
		return fiber.StatusNotFound, "not_found", "Не найдено"
	case errors.Is(err, issue.ErrAlreadyJoined):
		return fiber.StatusConflict, "already_joined", "Вы уже присоединились к этой заявке"
	case errors.Is(err, issue.ErrClosed):
		return fiber.StatusConflict, "issue_closed", "Заявка уже закрыта"
	case errors.Is(err, issue.ErrTransition):
		return fiber.StatusConflict, "invalid_transition", "Такой переход статуса невозможен"
	case errors.Is(err, issue.ErrNotDone):
		return fiber.StatusConflict, "not_done", "УК ещё не отметила заявку выполненной"
	case errors.Is(err, issue.ErrWindowClosed):
		return fiber.StatusConflict, "window_closed", "Ответить можно в течение 7 дней после отметки о выполнении"
	case errors.Is(err, issue.ErrAlreadyAnswered):
		return fiber.StatusConflict, "already_answered", "Вы уже ответили по этому ремонту"
	case errors.Is(err, issue.ErrAlreadyRated):
		return fiber.StatusConflict, "already_rated", "Вы уже оценили этот ремонт"
	case errors.Is(err, issue.ErrNotConfirmed):
		return fiber.StatusConflict, "not_confirmed", "Оценить можно ремонт, который вы подтвердили"
	case errors.Is(err, issue.ErrCommentRequired):
		return fiber.StatusUnprocessableEntity, "comment_required", "Напишите, что осталось не так"
	case errors.Is(err, appeal.ErrNotOverdue):
		return fiber.StatusConflict, "not_overdue", "Срок ответа ещё не истёк"
	case errors.Is(err, photos.ErrTooMany):
		return fiber.StatusConflict, "too_many_photos", fmt.Sprintf("К заявке можно приложить не больше %d фото", photos.MaxPerIssue)
	case errors.Is(err, photos.ErrNotImage):
		return fiber.StatusUnsupportedMediaType, "unsupported_media", "Нужна фотография в формате JPEG или PNG"
	case errors.Is(err, photos.ErrTooLarge):
		return fiber.StatusRequestEntityTooLarge, "photo_too_large", "Фото слишком большое: нужно до 5 МБ и до 40 мегапикселей"
	case errors.Is(err, auth.ErrInvalidContact):
		return fiber.StatusUnprocessableEntity, "invalid_contact", "Не удалось подтвердить номер. Попробуйте ещё раз"
	case errors.Is(err, issue.ErrReasonRequired):
		return fiber.StatusUnprocessableEntity, "reason_required", "Укажите причину отказа"
	case errors.Is(err, app.ErrInvalidInput), errors.Is(err, issue.ErrInvalid):
		return fiber.StatusUnprocessableEntity, "invalid_input", "Проверьте введённые данные"
	}
	if fe, ok := errors.AsType[*fiber.Error](err); ok {
		// Тело больше лимита Fiber: так бывает только при загрузке фото.
		if fe.Code == fiber.StatusRequestEntityTooLarge {
			return fe.Code, "photo_too_large", "Файлы слишком большие: не больше 5 МБ каждый"
		}
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
