// Команда api собирает сервис: конфигурацию, зависимости, бота и фоновые задачи.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"dommax/internal/storage/maxapi"
	"dommax/internal/transport/bot"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.BotMode == "off" {
		log.Info("bot disabled, nothing else to run yet")
		<-ctx.Done()
		return nil
	}

	if cfg.MaxInsecureTLS {
		log.Warn("TLS verification for MAX API is disabled; use only on a developer machine")
	}
	hc, err := maxapi.HTTPClient(cfg.MaxCAFile, cfg.MaxInsecureTLS)
	if err != nil {
		return err
	}
	maxClient := maxapi.New(cfg.MaxAPIURL, cfg.BotToken, hc)
	me, err := maxClient.Me(ctx)
	if err != nil {
		return err
	}
	log.Info("bot authorized", "username", me.Username, "mode", cfg.BotMode)

	h := bot.NewHandler(maxClient, me.Username, log)
	return bot.NewPoller(maxClient, h, log).Run(ctx)
}
