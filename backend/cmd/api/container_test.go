//go:build integration

package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dommax/internal/storage/postgres/pgtest"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testConfig(t *testing.T) config {
	t.Helper()
	dsn, drop, err := pgtest.Fresh(t.Context())
	if err != nil {
		t.Fatalf("test database: %v", err)
	}
	t.Cleanup(drop)
	return config{
		DatabaseURL: dsn, HTTPAddr: "127.0.0.1:0", SessionSecret: "0123456789abcdef",
		ConsentVersion: "v1", BotMode: "off", DemoAuth: true,
	}
}

// fakeMax — поддельный Bot API: отвечает на /me, считает вызовы команд и подписки.
type fakeMax struct {
	commands, subscriptions atomic.Int32
}

func (f *fakeMax) start(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me":
			io.WriteString(w, `{"user_id":1,"is_bot":true,"username":"test_bot"}`)
		case "/me/commands":
			f.commands.Add(1)
			io.WriteString(w, `{"commands":[]}`)
		case "/subscriptions":
			f.subscriptions.Add(1)
			io.WriteString(w, `{"success":true}`)
		case "/updates":
			// Настоящий long polling ждёт события; здесь короткая пауза, чтобы цикл не крутился впустую.
			select {
			case <-r.Context().Done():
			case <-time.After(50 * time.Millisecond):
			}
			io.WriteString(w, `{"updates":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestContainerGivesSingletonsAndServesHealth(t *testing.T) {
	c, err := Open(t.Context(), testConfig(t), quietLog())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()

	if c.Auth() != c.Auth() || c.Issues() != c.Issues() || c.Houses() != c.Houses() {
		t.Fatal("services must be created once")
	}
	app, err := c.HTTP()
	if err != nil {
		t.Fatalf("HTTP: %v", err)
	}
	again, _ := c.HTTP()
	if app != again {
		t.Fatal("HTTP app must be created once")
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/health", http.NoBody))
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("health: %v, %v", res, err)
	}
}

func TestOpenReportsUnreachableDatabase(t *testing.T) {
	cfg := config{DatabaseURL: "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable&connect_timeout=1"}
	_, err := Open(t.Context(), cfg, quietLog())
	if err == nil || !strings.Contains(err.Error(), "database") {
		t.Fatalf("err = %v, want database error", err)
	}
}

func TestRunWithPollingBotRegistersCommandsAndStops(t *testing.T) {
	fm := &fakeMax{}
	cfg := testConfig(t)
	cfg.BotMode, cfg.BotToken, cfg.MaxAPIURL = "polling", "token", fm.start(t)

	ctx, cancel := context.WithCancel(t.Context())
	c, err := Open(ctx, cfg, quietLog())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()

	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for fm.commands.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if fm.commands.Load() == 0 {
		t.Fatal("bot commands were not registered")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after cancel = %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestWebhookModeSubscribesAndChecksSecret(t *testing.T) {
	fm := &fakeMax{}
	cfg := testConfig(t)
	cfg.BotMode, cfg.BotToken, cfg.MaxAPIURL = "webhook", "token", fm.start(t)
	cfg.Domain, cfg.WebhookSecret = "dom.example", "hook_secret"

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c, err := Open(ctx, cfg, quietLog())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()
	go func() { _ = c.Run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	for fm.subscriptions.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if fm.subscriptions.Load() == 0 {
		t.Fatal("webhook was not subscribed")
	}
	app, _ := c.HTTP()
	req := httptest.NewRequest(http.MethodPost, "/webhook/max", strings.NewReader(`{"update_type":"bot_started"}`))
	req.Header.Set("X-Max-Bot-Api-Secret", "wrong")
	res, err := app.Test(req)
	if err != nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("webhook with wrong secret: %v, %v", res, err)
	}
}
