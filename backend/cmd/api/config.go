package main

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"dommax/internal/storage/files"
	"dommax/internal/storage/maxapi"
)

type config struct {
	DatabaseURL    string
	HTTPAddr       string
	SessionSecret  string
	DemoAuth       bool   // POST /api/v1/auth/demo для проверяющих
	ConsentVersion string // версия согласия на обработку ПДн

	BotMode        string // off | polling | webhook
	BotToken       string
	Domain         string // для URL webhook: https://<DOMAIN>/webhook/max
	WebhookSecret  string
	MaxAPIURL      string
	MaxCAFile      string
	MaxInsecureTLS bool

	S3          files.S3Config // фото в S3 (MinIO в Docker); без Endpoint — на диске в PhotosDir
	PhotosDir   string         // запасное хранилище фото для локального запуска
	GeocoderURL string         // API, совместимое с Nominatim (ADR-016); пусто — геокодер выключен
	GeocoderUA  string         // User-Agent с названием и контактом: требование правил Nominatim
	OllamaURL   string         // пусто — LLM не подключена, подсказка по ключевым словам
	OllamaModel string
}

func (c config) WebhookURL() string { return "https://" + c.Domain + "/webhook/max" }

// Формат секрета webhook задан документацией POST /subscriptions.
var webhookSecretRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{5,256}$`)

// loadConfig читает переменные окружения через getenv и собирает все ошибки сразу.
func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		DatabaseURL:    getenv("DATABASE_URL"),
		HTTPAddr:       cmp.Or(getenv("HTTP_ADDR"), ":8080"),
		SessionSecret:  getenv("SESSION_SECRET"),
		ConsentVersion: cmp.Or(getenv("CONSENT_VERSION"), "v1"),
		BotMode:        cmp.Or(getenv("BOT_MODE"), "off"),
		BotToken:       getenv("MAX_BOT_TOKEN"),
		Domain:         getenv("DOMAIN"),
		WebhookSecret:  getenv("MAX_WEBHOOK_SECRET"),
		MaxAPIURL:      cmp.Or(getenv("MAX_API_URL"), maxapi.DefaultBaseURL),
		MaxCAFile:      getenv("MAX_API_CA_FILE"),
		S3: files.S3Config{
			Endpoint:  getenv("S3_ENDPOINT"),
			AccessKey: getenv("S3_ACCESS_KEY"),
			SecretKey: getenv("S3_SECRET_KEY"),
			Bucket:    cmp.Or(getenv("S3_BUCKET"), "photos"),
		},
		PhotosDir:   cmp.Or(getenv("PHOTOS_DIR"), "data/photos"),
		GeocoderURL: getenv("GEOCODER_URL"),
		GeocoderUA:  cmp.Or(getenv("GEOCODER_USER_AGENT"), "dom-max/1.0"),
		OllamaURL:   getenv("OLLAMA_URL"),
		OllamaModel: cmp.Or(getenv("OLLAMA_MODEL"), "qwen3:4b"),
	}
	var errs []error
	parseBool := func(name string, dst *bool) {
		if v := getenv(name); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
			}
			*dst = b
		}
	}
	parseBool("DEMO_AUTH_ENABLED", &c.DemoAuth)
	parseBool("MAX_API_INSECURE_TLS", &c.MaxInsecureTLS)
	parseBool("S3_USE_SSL", &c.S3.UseSSL)

	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(c.SessionSecret) < 16 {
		errs = append(errs, errors.New("SESSION_SECRET must be at least 16 characters"))
	}
	if c.S3.Endpoint != "" {
		if c.S3.AccessKey == "" {
			errs = append(errs, errors.New("S3_ACCESS_KEY is required with S3_ENDPOINT"))
		}
		if c.S3.SecretKey == "" {
			errs = append(errs, errors.New("S3_SECRET_KEY is required with S3_ENDPOINT"))
		}
	}
	switch c.BotMode {
	case "off":
	case "polling", "webhook":
		if c.BotToken == "" {
			errs = append(errs, fmt.Errorf("MAX_BOT_TOKEN is required for BOT_MODE=%s", c.BotMode))
		}
	default:
		errs = append(errs, fmt.Errorf("BOT_MODE=%q: want off, polling or webhook", c.BotMode))
	}
	if c.BotMode == "webhook" {
		if c.Domain == "" {
			errs = append(errs, errors.New("DOMAIN is required for BOT_MODE=webhook"))
		}
		if !webhookSecretRe.MatchString(c.WebhookSecret) {
			errs = append(errs, errors.New("MAX_WEBHOOK_SECRET must match [a-zA-Z0-9_-]{5,256}"))
		}
	}
	return c, errors.Join(errs...)
}
