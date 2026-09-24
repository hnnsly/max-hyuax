package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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

const sessionTTL = 12 * time.Hour

// Container — DI-контейнер сервиса: единственное место, где зависимости собираются вместе
// (ADR-010). База открывается сразу в Open, остальное создаётся лениво при первом запросе
// и живёт один экземпляр на процесс.
type Container struct {
	cfg   config
	log   *slog.Logger
	ctx   context.Context // корневой контекст для провайдеров, которым нужна сеть
	store *postgres.Store

	auth       func() *auth.Service
	issues     func() *issues.Service
	houses     func() *houses.Service
	maxClient  func() (*maxapi.Client, error)
	botMe      func() (maxapi.User, error)
	botHandler func() (*bot.Handler, error)
	webhook    func() (*bot.Webhook, error)
	cards      func() (*cards.Service, error)
	outbox     func() (*jobs.OutboxWorker, error)
	http       func() (*fiber.App, error)
}

// Open накатывает миграции, открывает базу и описывает, как создать остальные зависимости.
func Open(ctx context.Context, cfg config, log *slog.Logger) (*Container, error) {
	if err := postgres.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return nil, fmt.Errorf("database migrate: %w", err)
	}
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("database open: %w", err)
	}
	c := &Container{cfg: cfg, log: log, ctx: ctx, store: store}

	c.auth = sync.OnceValue(func() *auth.Service {
		return auth.NewService(store, auth.Config{
			BotToken: cfg.BotToken, SessionSecret: cfg.SessionSecret, SessionTTL: sessionTTL,
			DemoEnabled: cfg.DemoAuth, Now: time.Now,
		})
	})
	c.issues = sync.OnceValue(func() *issues.Service {
		return issues.NewService(store, issues.Config{
			Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: cfg.ConsentVersion,
		})
	})
	c.houses = sync.OnceValue(func() *houses.Service { return houses.NewService(store) })

	c.maxClient = sync.OnceValues(func() (*maxapi.Client, error) {
		if cfg.MaxInsecureTLS {
			log.Warn("TLS verification for MAX API is disabled; use only on a developer machine")
		}
		hc, err := maxapi.HTTPClient(cfg.MaxCAFile, cfg.MaxInsecureTLS)
		if err != nil {
			return nil, err
		}
		return maxapi.New(cfg.MaxAPIURL, cfg.BotToken, hc), nil
	})
	c.botMe = sync.OnceValues(func() (maxapi.User, error) {
		client, err := c.maxClient()
		if err != nil {
			return maxapi.User{}, err
		}
		me, err := client.Me(c.ctx)
		if err != nil {
			return maxapi.User{}, fmt.Errorf("bot identity: %w", err)
		}
		return me, nil
	})
	c.botHandler = sync.OnceValues(func() (*bot.Handler, error) {
		client, me, err := c.bot()
		if err != nil {
			return nil, err
		}
		return bot.NewHandler(client, me.Username, bot.Services{
			Auth: c.Auth(), Issues: c.Issues(), Houses: c.Houses(), ConsentVersion: cfg.ConsentVersion, Now: time.Now,
		}, log), nil
	})
	c.webhook = sync.OnceValues(func() (*bot.Webhook, error) {
		h, err := c.botHandler()
		if err != nil {
			return nil, err
		}
		return bot.NewWebhook(cfg.WebhookSecret, h, store, log), nil
	})
	c.cards = sync.OnceValues(func() (*cards.Service, error) {
		client, me, err := c.bot()
		if err != nil {
			return nil, err
		}
		return cards.NewService(store, bot.NewCardSender(client, me.Username), time.Now), nil
	})
	c.outbox = sync.OnceValues(func() (*jobs.OutboxWorker, error) {
		svc, err := c.cards()
		if err != nil {
			return nil, err
		}
		limiter := jobs.NewLimiter(jobs.GlobalInterval, jobs.PerChatInterval, time.Now, jobs.SleepCtx)
		return jobs.NewOutboxWorker(store.Outbox(), svc.Deliver, limiter, log), nil
	})
	c.http = sync.OnceValues(func() (*fiber.App, error) {
		deps := httpapi.Deps{
			Auth: c.Auth(), Issues: c.Issues(), Houses: c.Houses(),
			Ping: store.Ping, ConsentVersion: cfg.ConsentVersion, Now: time.Now, Log: log,
		}
		if cfg.BotMode == "webhook" {
			wh, err := c.webhook()
			if err != nil {
				return nil, err
			}
			deps.Webhook = wh
		}
		return httpapi.New(deps), nil
	})
	return c, nil
}

func (c *Container) Close() { c.store.Close() }

func (c *Container) Store() *postgres.Store  { return c.store }
func (c *Container) Auth() *auth.Service     { return c.auth() }
func (c *Container) Issues() *issues.Service { return c.issues() }
func (c *Container) Houses() *houses.Service { return c.houses() }

func (c *Container) MaxClient() (*maxapi.Client, error)        { return c.maxClient() }
func (c *Container) BotIdentity() (maxapi.User, error)         { return c.botMe() }
func (c *Container) BotHandler() (*bot.Handler, error)         { return c.botHandler() }
func (c *Container) Webhook() (*bot.Webhook, error)            { return c.webhook() }
func (c *Container) Cards() (*cards.Service, error)            { return c.cards() }
func (c *Container) OutboxWorker() (*jobs.OutboxWorker, error) { return c.outbox() }
func (c *Container) HTTP() (*fiber.App, error)                 { return c.http() }

// bot — клиент Bot API и имя бота: нужны и диалогу, и живым карточкам.
func (c *Container) bot() (*maxapi.Client, maxapi.User, error) {
	client, err := c.maxClient()
	if err != nil {
		return nil, maxapi.User{}, err
	}
	me, err := c.botMe()
	return client, me, err
}
