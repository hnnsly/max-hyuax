package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"strconv"

	"dommax/internal/storage/maxapi"
)

type config struct {
	BotToken       string
	BotMode        string // polling | off; webhook появится вместе с HTTP-сервером
	MaxAPIURL      string
	MaxCAFile      string
	MaxInsecureTLS bool
}

func loadConfig() (config, error) {
	c := config{
		BotToken:  os.Getenv("MAX_BOT_TOKEN"),
		BotMode:   cmp.Or(os.Getenv("BOT_MODE"), "polling"),
		MaxAPIURL: cmp.Or(os.Getenv("MAX_API_URL"), maxapi.DefaultBaseURL),
		MaxCAFile: os.Getenv("MAX_API_CA_FILE"),
	}
	var errs []error
	if v := os.Getenv("MAX_API_INSECURE_TLS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("MAX_API_INSECURE_TLS: %w", err))
		}
		c.MaxInsecureTLS = b
	}
	switch c.BotMode {
	case "polling":
		if c.BotToken == "" {
			errs = append(errs, errors.New("MAX_BOT_TOKEN is required for BOT_MODE=polling"))
		}
	case "off":
	default:
		errs = append(errs, fmt.Errorf("BOT_MODE=%q: want polling or off", c.BotMode))
	}
	return c, errors.Join(errs...)
}
