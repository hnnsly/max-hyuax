package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"uuid"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app"
	"dommax/internal/app/appeal"
	"dommax/internal/app/auth"
	"dommax/internal/app/cards"
	"dommax/internal/app/council"
	"dommax/internal/app/hints"
	"dommax/internal/app/houses"
	"dommax/internal/app/issues"
	"dommax/internal/app/photos"
	"dommax/internal/storage/files"
	"dommax/internal/storage/geo"
	"dommax/internal/storage/llm"
	"dommax/internal/storage/maxapi"
	"dommax/internal/storage/postgres"
	"dommax/internal/transport/bot"
	"dommax/internal/transport/httpapi"
	"dommax/internal/transport/jobs"
)

const (
	sessionTTL = 12 * time.Hour
	// llmTimeout — сколько ждать подсказку модели; дальше работают ключевые слова (ADR-008).
	llmTimeout = 8 * time.Second
	// appealLinkTTL — сколько действует ссылка на PDF-обращение.
	appealLinkTTL = 10 * time.Minute
)

// Container — DI-контейнер сервиса: единственное место, где зависимости собираются вместе
// (ADR-010). База открывается сразу в Open, остальное создаётся лениво при первом запросе
// и живёт один экземпляр на процесс.
type Container struct {
	cfg   config
	log   *slog.Logger
	ctx   context.Context // корневой контекст для провайдеров, которым нужна сеть
	store *postgres.Store
	files fileStore

	auth       func() *auth.Service
	issues     func() *issues.Service
	houses     func() *houses.Service
	hints      func() *hints.Service
	photos     func() *photos.Service
	geocoder   func() app.Geocoder
	council    func() *council.Service
	maxClient  func() (*maxapi.Client, error)
	botMe      func() (maxapi.User, error)
	botHandler func() (*bot.Handler, error)
	webhook    func() (*bot.Webhook, error)
	cards      func() (*cards.Service, error)
	outbox     func() (*jobs.OutboxWorker, error)
	overdue    func() *jobs.OverdueJob
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
	photoFiles, err := openFiles(ctx, cfg, log)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("photos storage: %w", err)
	}
	c := &Container{cfg: cfg, log: log, ctx: ctx, store: store, files: photoFiles}

	c.auth = sync.OnceValue(func() *auth.Service {
		return auth.NewService(store, auth.Config{
			BotToken: cfg.BotToken, SessionSecret: cfg.SessionSecret, SessionTTL: sessionTTL,
			DemoEnabled: cfg.DemoAuth, ConsentVersion: cfg.ConsentVersion, Now: time.Now,
		})
	})
	c.issues = sync.OnceValue(func() *issues.Service {
		return issues.NewService(store, issues.Config{
			Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: cfg.ConsentVersion,
		})
	})
	c.geocoder = sync.OnceValue(func() app.Geocoder {
		// Без GEOCODER_URL импорт и «Найти дома рядом» работают без геокодера.
		if cfg.GeocoderURL == "" {
			return nil
		}
		n, err := geo.NewNominatim(cfg.GeocoderURL, cfg.GeocoderUA)
		if err != nil {
			log.Warn("geocoder disabled", "err", err)
			return nil
		}
		return n
	})
	c.houses = sync.OnceValue(func() *houses.Service { return houses.NewService(store, c.geocoder()) })
	c.council = sync.OnceValue(func() *council.Service {
		return council.NewService(store, council.Config{
			Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: cfg.ConsentVersion,
		})
	})
	c.photos = sync.OnceValue(func() *photos.Service {
		return photos.NewService(store, photoFiles, photos.Config{
			Now: time.Now, NewID: func() string { return uuid.NewV7().String() }, ConsentVersion: cfg.ConsentVersion,
		})
	})
	c.hints = sync.OnceValue(func() *hints.Service {
		// Без OLLAMA_URL подсказка работает на ключевых словах.
		var model hints.LLM
		if cfg.OllamaURL != "" {
			model = llm.NewOllama(cfg.OllamaURL, cfg.OllamaModel, &http.Client{})
			log.Info("llm hints enabled", "model", cfg.OllamaModel)
		}
		return hints.NewService(model, llmTimeout, log)
	})

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
		var me maxapi.User
		for attempt := 1; attempt <= 3; attempt++ {
			callCtx, cancel := context.WithTimeout(c.ctx, 7*time.Second)
			me, err = client.Me(callCtx)
			cancel()
			if err == nil {
				return me, nil
			}
			if attempt < 3 {
				log.Warn("bot authorization attempt failed, retrying...", "attempt", attempt, "err", err)
				time.Sleep(2 * time.Second)
			}
		}
		return maxapi.User{}, fmt.Errorf("bot identity: %w", err)
	})
	c.botHandler = sync.OnceValues(func() (*bot.Handler, error) {
		client, me, err := c.bot()
		if err != nil {
			return nil, err
		}
		cardSvc, err := c.cards()
		if err != nil {
			return nil, err
		}
		return bot.NewHandler(client, me.Username, bot.Services{
			Auth: c.Auth(), Issues: c.Issues(), Houses: c.Houses(), Hints: c.Hints(), Photos: c.Photos(),
			Cards: cardSvc, Council: c.council(), Pending: store.Pending(), DemoRoles: cfg.DemoAuth,
			ConsentVersion: cfg.ConsentVersion, Now: time.Now,
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
	c.overdue = sync.OnceValue(func() *jobs.OverdueJob {
		mark := c.Issues().MarkOverdue
		if cfg.DemoAuth {
			// На демо-стенде пример данных сдвигается к текущей дате до проверки сроков.
			mark = func(ctx context.Context) (int, error) {
				if days, err := store.ShiftSampleData(ctx, time.Now()); err != nil {
					log.WarnContext(ctx, "shift sample data failed", "err", err)
				} else if days > 0 {
					log.InfoContext(ctx, "sample data shifted", "days", days)
				}
				return c.Issues().MarkOverdue(ctx)
			}
		}
		return jobs.NewOverdueJob(mark, jobs.OverdueInterval, log)
	})
	c.http = sync.OnceValues(func() (*fiber.App, error) {
		deps := httpapi.Deps{
			Auth: c.Auth(), Issues: c.Issues(), Houses: c.Houses(), Hints: c.Hints(),
			Appeal:  appeal.NewService(store, appeal.Config{Secret: []byte(cfg.SessionSecret), TTL: appealLinkTTL, Now: time.Now}),
			Photos:  c.Photos(),
			Council: c.council(),
			Ping:    store.Ping, ConsentVersion: cfg.ConsentVersion, Now: time.Now, Log: log,
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

// fileStore — хранилище фото, которое закрывается вместе с контейнером.
type fileStore interface {
	app.FileStore
	Close() error
}

// openFiles выбирает хранилище фото: S3 (MinIO), если он настроен, иначе каталог на диске.
// Хранилище проверяется при старте: без него загрузка фото сломается уже у жителя.
func openFiles(ctx context.Context, cfg config, log *slog.Logger) (fileStore, error) {
	if cfg.S3.Endpoint != "" {
		s3, err := files.NewS3(ctx, cfg.S3)
		if err != nil {
			return nil, err
		}
		log.Info("photos storage", "kind", "s3", "endpoint", cfg.S3.Endpoint, "bucket", cfg.S3.Bucket)
		return s3, nil
	}
	disk, err := files.NewDisk(cfg.PhotosDir)
	if err != nil {
		return nil, err
	}
	log.Info("photos storage", "kind", "disk", "dir", cfg.PhotosDir)
	return disk, nil
}

func (c *Container) Close() {
	c.store.Close()
	_ = c.files.Close()
}

func (c *Container) Store() *postgres.Store  { return c.store }
func (c *Container) Auth() *auth.Service     { return c.auth() }
func (c *Container) Issues() *issues.Service { return c.issues() }
func (c *Container) Houses() *houses.Service { return c.houses() }
func (c *Container) Hints() *hints.Service   { return c.hints() }
func (c *Container) Photos() *photos.Service { return c.photos() }

// Importer — импорт реестра домов (команда import-houses).
func (c *Container) Importer() *houses.Importer { return houses.NewImporter(c.store, c.geocoder()) }

func (c *Container) MaxClient() (*maxapi.Client, error)        { return c.maxClient() }
func (c *Container) BotIdentity() (maxapi.User, error)         { return c.botMe() }
func (c *Container) BotHandler() (*bot.Handler, error)         { return c.botHandler() }
func (c *Container) Webhook() (*bot.Webhook, error)            { return c.webhook() }
func (c *Container) Cards() (*cards.Service, error)            { return c.cards() }
func (c *Container) OutboxWorker() (*jobs.OutboxWorker, error) { return c.outbox() }
func (c *Container) OverdueJob() *jobs.OverdueJob              { return c.overdue() }
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
