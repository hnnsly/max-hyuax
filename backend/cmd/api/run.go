package main

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/storage/maxapi"
	"dommax/internal/transport/bot"
)

const shutdownTimeout = 10 * time.Second

// Run запускает бота по BOT_MODE, фоновые задачи и HTTP и ждёт остановки по ctx.
func (c *Container) Run(ctx context.Context) error {
	if c.cfg.DemoAuth {
		c.log.Warn("demo login is enabled: use only on the demo stand")
	}
	// Модель подсказки загружается в память заранее; пока она не готова, работают ключевые слова.
	go c.Hints().WarmUp(ctx)
	// Просрочка отмечается при любом BOT_MODE: уведомления ждут в outbox, пока бот выключен.
	go func() {
		if err := c.OverdueJob().Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.log.Error("overdue job stopped", "err", err)
		}
	}()
	poll, err := c.startBot(ctx)
	if err != nil {
		return err
	}
	srv, err := c.HTTP()
	if err != nil {
		return err
	}
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- srv.Listen(c.cfg.HTTPAddr, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	c.log.Info("api started", "addr", c.cfg.HTTPAddr, "bot_mode", c.cfg.BotMode)

	select {
	case err = <-listenErr:
	case err = <-poll:
	case <-ctx.Done():
	}
	if shutdownErr := srv.ShutdownWithTimeout(shutdownTimeout); shutdownErr != nil {
		c.log.Warn("http shutdown", "err", shutdownErr)
	}
	if c.cfg.BotMode == "webhook" {
		if wh, whErr := c.Webhook(); whErr == nil {
			wh.Wait()
		}
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// startBot включает бота. Для polling возвращает канал с ошибкой цикла опроса.
func (c *Container) startBot(ctx context.Context) (<-chan error, error) {
	if c.cfg.BotMode == "off" {
		return nil, nil
	}
	client, me, err := c.bot()
	if err != nil {
		return nil, err
	}
	c.log.Info("bot authorized", "username", me.Username)
	// Меню команд бота; без него бот тоже работает, поэтому ошибка только в лог.
	if err := client.SetCommands(ctx, bot.Commands); err != nil {
		c.log.Warn("set bot commands failed", "err", err)
	}

	// Живые карточки: очередь outbox отправляется в фоне с лимитами Bot API.
	worker, err := c.OutboxWorker()
	if err != nil {
		return nil, err
	}
	go func() {
		if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			c.log.Error("outbox worker stopped", "err", err)
		}
	}()

	h, err := c.BotHandler()
	if err != nil {
		return nil, err
	}
	if c.cfg.BotMode == "webhook" {
		types := []maxapi.UpdateType{maxapi.UpdateBotStarted, maxapi.UpdateMessageCreated, maxapi.UpdateMessageCallback}
		var subErr error
		for attempt := 1; attempt <= 3; attempt++ {
			subCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
			subErr = client.Subscribe(subCtx, c.cfg.WebhookURL(), c.cfg.WebhookSecret, types)
			cancel()
			if subErr == nil {
				break
			}
			if attempt < 3 {
				c.log.Warn("webhook subscription attempt failed, retrying...", "attempt", attempt, "err", subErr)
				time.Sleep(2 * time.Second)
			}
		}
		if subErr != nil {
			return nil, subErr
		}
		c.log.Info("webhook subscribed", "url", c.cfg.WebhookURL())
		return nil, nil
	}
	done := make(chan error, 1)
	go func() { done <- bot.NewPoller(client, h, c.log).Run(ctx) }()
	return done, nil
}
