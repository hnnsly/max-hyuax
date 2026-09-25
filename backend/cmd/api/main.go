// Команда api — точка входа сервиса: конфигурация из окружения, DI-контейнер, запуск.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
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

	// Без аргументов — сервис; import-houses — разовая загрузка реестра домов (ADR-016).
	if args := os.Args[1:]; len(args) > 0 {
		if len(args) != 2 || args[0] != "import-houses" {
			return errors.New("usage: api [import-houses <file.csv|->]")
		}
		// Отчёт идёт в stdout, служебные логи — в stderr, чтобы не смешивались.
		return importHouses(ctx, cfg, slog.New(slog.NewJSONHandler(os.Stderr, nil)), args[1], os.Stdout)
	}

	c, err := Open(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Run(ctx)
}
