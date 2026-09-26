package main

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func base() map[string]string {
	return map[string]string{"DATABASE_URL": "postgres://x", "SESSION_SECRET": "0123456789abcdef"}
}

func TestConfigDefaultsWithBotOff(t *testing.T) {
	c, err := loadConfig(env(base()))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	// Роль для проверки включена по умолчанию: комиссия проходит кабинеты и без демо-входа.
	if c.BotMode != "off" || c.HTTPAddr != ":8080" || c.ConsentVersion != "v1" || c.DemoAuth || !c.RoleSwitch {
		t.Fatalf("config = %+v", c)
	}
}

func TestConfigRequiresSecrets(t *testing.T) {
	_, err := loadConfig(env(map[string]string{"SESSION_SECRET": "short"}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("err = %v", err)
	}
}

func TestConfigSeparateDatabaseParams(t *testing.T) {
	m := map[string]string{
		"POSTGRES_HOST":     "db.internal",
		"POSTGRES_PORT":     "5433",
		"POSTGRES_USER":     "myuser",
		"POSTGRES_PASSWORD": "mypassword",
		"POSTGRES_DB":       "mydb",
		"POSTGRES_SSLMODE":  "require",
		"SESSION_SECRET":    "0123456789abcdef",
	}
	c, err := loadConfig(env(m))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	want := "postgres://myuser:mypassword@db.internal:5433/mydb?sslmode=require"
	if c.DatabaseURL != want {
		t.Fatalf("got DatabaseURL = %q, want %q", c.DatabaseURL, want)
	}

	// Проверка с префиксом DB_*
	m2 := map[string]string{
		"DB_HOST":        "127.0.0.1",
		"DB_PORT":        "5432",
		"DB_USER":        "user2",
		"DB_PASSWORD":    "pass2",
		"DB_NAME":        "dommax",
		"SESSION_SECRET": "0123456789abcdef",
	}
	c2, err := loadConfig(env(m2))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	want2 := "postgres://user2:pass2@127.0.0.1:5432/dommax?sslmode=disable"
	if c2.DatabaseURL != want2 {
		t.Fatalf("got DatabaseURL = %q, want %q", c2.DatabaseURL, want2)
	}
}

// Без S3_ENDPOINT фото пишутся на диск; с ним нужны ключи доступа, бакет по умолчанию photos.
func TestConfigS3(t *testing.T) {
	c, err := loadConfig(env(base()))
	if err != nil || c.S3.Endpoint != "" {
		t.Fatalf("default S3 = %+v, err = %v", c.S3, err)
	}
	m := base()
	m["S3_ENDPOINT"] = "minio:9000"
	m["S3_USE_SSL"] = "maybe"
	_, err = loadConfig(env(m))
	for _, want := range []string{"S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_USE_SSL"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want mention of %s", err, want)
		}
	}
	m["S3_ACCESS_KEY"], m["S3_SECRET_KEY"], m["S3_USE_SSL"] = "user", "secret", "true"
	c, err = loadConfig(env(m))
	if err != nil || c.S3.Bucket != "photos" || !c.S3.UseSSL || c.S3.AccessKey != "user" {
		t.Fatalf("S3 = %+v, err = %v", c.S3, err)
	}
}

func TestWebhookModeNeedsDomainTokenAndValidSecret(t *testing.T) {
	m := base()
	m["BOT_MODE"] = "webhook"
	m["MAX_WEBHOOK_SECRET"] = "bad secret!"
	_, err := loadConfig(env(m))
	for _, want := range []string{"MAX_BOT_TOKEN", "DOMAIN", "MAX_WEBHOOK_SECRET"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want mention of %s", err, want)
		}
	}
	m["MAX_BOT_TOKEN"], m["DOMAIN"], m["MAX_WEBHOOK_SECRET"] = "tok", "dom.example", "good_secret-1"
	c, err := loadConfig(env(m))
	if err != nil || c.WebhookURL() != "https://dom.example/webhook/max" {
		t.Fatalf("config = %+v, err = %v", c, err)
	}
}
