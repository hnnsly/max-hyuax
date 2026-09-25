package main

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

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

// buildDatabaseURL собирает DSN из отдельных переменных (POSTGRES_*/DB_*) или возвращает DATABASE_URL.
func buildDatabaseURL(getenv func(string) string) string {
	clean := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.Trim(strings.TrimSpace(getenv(k)), `"'`); v != "" {
				return v
			}
		}
		return ""
	}

	host := clean("POSTGRES_HOST", "POSTGRESQL_HOST", "DB_HOST")
	user := clean("POSTGRES_USER", "POSTGRESQL_USER", "DB_USER")
	pass := clean("POSTGRES_PASSWORD", "POSTGRESQL_PASSWORD", "DB_PASSWORD")
	name := clean("POSTGRES_DB", "POSTGRESQL_DB", "DB_NAME")

	if host != "" || user != "" || name != "" {
		host = cmp.Or(host, "localhost")
		port := cmp.Or(clean("POSTGRES_PORT", "POSTGRESQL_PORT", "DB_PORT"), "5432")
		user = cmp.Or(user, "dommax")
		name = cmp.Or(name, "dommax")
		ssl := clean("POSTGRES_SSLMODE", "POSTGRES_SSL", "DB_SSLMODE")
		if ssl == "" || ssl == "false" {
			ssl = "disable"
		} else if ssl == "true" {
			ssl = "require"
		}

		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(user, pass),
			Host:   fmt.Sprintf("%s:%s", host, port),
			Path:   name,
		}
		q := u.Query()
		q.Set("sslmode", ssl)
		u.RawQuery = q.Encode()
		return u.String()
	}

	return clean("DATABASE_URL")
}

// loadConfig читает переменные окружения через getenv и собирает все ошибки сразу.
func loadConfig(getenv func(string) string) (config, error) {
	rawS3 := cmp.Or(getenv("S3_ENDPOINT"), getenv("MINIO_ENDPOINT"))
	useSSL := getenv("S3_USE_SSL") == "true" || getenv("MINIO_USE_SSL") == "true" || getenv("USE_SSL") == "true" ||
		strings.HasPrefix(rawS3, "https://") || strings.HasSuffix(rawS3, ":443")

	c := config{
		DatabaseURL:    buildDatabaseURL(getenv),
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
			Endpoint:  rawS3,
			AccessKey: cmp.Or(getenv("S3_ACCESS_KEY"), getenv("MINIO_ACCESS_KEY_ID"), getenv("MINIO_ROOT_USER")),
			SecretKey: cmp.Or(getenv("S3_SECRET_KEY"), getenv("MINIO_SECRET_ACCESS_KEY"), getenv("MINIO_ROOT_PASSWORD")),
			Bucket:    cmp.Or(getenv("S3_BUCKET"), getenv("MINIO_BUCKET"), "photos"),
			UseSSL:    useSSL,
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
	if getenv("S3_USE_SSL") != "" {
		parseBool("S3_USE_SSL", &c.S3.UseSSL)
	}

	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL or POSTGRES_HOST/DB_HOST is required"))
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
