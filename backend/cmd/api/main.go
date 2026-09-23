// Команда api собирает сервис: конфигурацию, зависимости, HTTP, бота и фоновые задачи.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"uuid"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app/auth"
	"dommax/internal/app/cards"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/storage/maxapi"
	"dommax/internal/storage/postgres"
	"dommax/internal/transport/bot"
	"dommax/internal/transport/httpapi"
	"dommax/internal/transport/jobs"
)

const (
	sessionTTL      = 12 * time.Hour
	shutdownTimeout = 10 * time.Second
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := postgres.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	deps := httpapi.Deps{
		Auth: auth.NewService(store, auth.Config{
			BotToken: cfg.BotToken, SessionSecret: cfg.SessionSecret, SessionTTL: sessionTTL,
			DemoEnabled: cfg.DemoAuth, Now: time.Now,
		}),
		Issues: issues.NewService(store, issues.Config{
			Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: cfg.ConsentVersion,
		}),
		Houses:         houses.NewService(store),
		Ping:           store.Ping,
		ConsentVersion: cfg.ConsentVersion,
		Now:            time.Now,
		Log:            log,
	}
	if cfg.DemoAuth {
		log.Warn("demo login is enabled: use only on the demo stand")
	}

	poll, err := startBot(ctx, cfg, store, &deps, log)
	if err != nil {
		return err
	}

	srv := httpapi.New(deps)
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- srv.Listen(cfg.HTTPAddr, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	log.Info("api started", "addr", cfg.HTTPAddr, "bot_mode", cfg.BotMode)

	select {
	case err = <-listenErr:
	case err = <-poll:
	case <-ctx.Done():
	}
	if shutdownErr := srv.ShutdownWithTimeout(shutdownTimeout); shutdownErr != nil {
		log.Warn("http shutdown", "err", shutdownErr)
	}
	if deps.Webhook != nil {
		deps.Webhook.Wait()
	}
	return err
}

// startBot поднимает бота по BOT_MODE. Для polling возвращает канал с ошибкой цикла опроса.
func startBot(ctx context.Context, cfg config, store *postgres.Store, deps *httpapi.Deps, log *slog.Logger) (<-chan error, error) {
	if cfg.BotMode == "off" {
		return nil, nil
	}
	if cfg.MaxInsecureTLS {
		log.Warn("TLS verification for MAX API is disabled; use only on a developer machine")
	}
	hc, err := maxapi.HTTPClient(cfg.MaxCAFile, cfg.MaxInsecureTLS)
	if err != nil {
		return nil, err
	}
	client := maxapi.New(cfg.MaxAPIURL, cfg.BotToken, hc)
	me, err := client.Me(ctx)
	if err != nil {
		return nil, err
	}
	log.Info("bot authorized", "username", me.Username)
	h := bot.NewHandler(client, me.Username, bot.Services{
		Auth: deps.Auth, Issues: deps.Issues, Houses: deps.Houses, ConsentVersion: cfg.ConsentVersion, Now: time.Now,
	}, log)
	// Меню команд бота; без него бот тоже работает, поэтому ошибка только в лог.
	if err := client.SetCommands(ctx, bot.Commands); err != nil {
		log.Warn("set bot commands failed", "err", err)
	}

	// Живые карточки: очередь outbox отправляется в фоне с лимитами Bot API.
	cardSvc := cards.NewService(store, bot.NewCardSender(client, me.Username), time.Now)
	limiter := jobs.NewLimiter(jobs.GlobalInterval, jobs.PerChatInterval, time.Now, jobs.SleepCtx)
	go func() {
		if err := jobs.NewOutboxWorker(store.Outbox(), cardSvc.Deliver, limiter, log).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("outbox worker stopped", "err", err)
		}
	}()

	if cfg.BotMode == "webhook" {
		deps.Webhook = bot.NewWebhook(cfg.WebhookSecret, h, store, log)
		types := []maxapi.UpdateType{maxapi.UpdateBotStarted, maxapi.UpdateMessageCreated, maxapi.UpdateMessageCallback}
		if err := client.Subscribe(ctx, cfg.WebhookURL(), cfg.WebhookSecret, types); err != nil {
			return nil, err
		}
		log.Info("webhook subscribed", "url", cfg.WebhookURL())
		return nil, nil
	}
	done := make(chan error, 1)
	go func() { done <- bot.NewPoller(client, h, log).Run(ctx) }()
	return done, nil
}
