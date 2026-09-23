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
	if c.BotMode != "off" || c.HTTPAddr != ":8080" || c.ConsentVersion != "v1" || c.DemoAuth {
		t.Fatalf("config = %+v", c)
	}
}

func TestConfigRequiresSecrets(t *testing.T) {
	_, err := loadConfig(env(map[string]string{"SESSION_SECRET": "short"}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("err = %v", err)
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
