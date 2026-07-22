package main

import (
	"context"

	"github.com/EgorKo25/EasyNote/src/background"
	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/logger"
	notificationtelegram "github.com/EgorKo25/EasyNote/src/notification/telegram"
	"github.com/EgorKo25/EasyNote/src/telegram"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		panic(err)
	}

	if err = logger.Initialize(cfg.AppConfig, cfg.Logger); err != nil {
		panic(err)
	}

	log := logger.Get("telegram-ui/main")

	defer func() { _ = log.Sync() }()

	log.With(logger.Any("config", cfg)).Info("the telegram ui configuration is initialized")

	ctx := context.Background()

	telegramSender, err := notificationtelegram.NewSender(ctx, cfg.Telegram)
	if err != nil {
		log.Error("cannot initialize telegram sender", err)
		panic(err)
	}

	backendClient, err := telegram.NewBackendClient(cfg.BackendBaseURL, cfg.RequestTimeout)
	if err != nil {
		log.Error("cannot initialize backend client", err)
		panic(err)
	}

	reminderBot := telegram.NewReminderBot(telegramSender, backendClient, backendClient, cfg.Telegram)

	runner := background.NewRunner()

	runner.Run(ctx, reminderBot)

	if err = runner.Wait(); err != nil {
		log.Error("telegram ui process failed", err)
		panic(err)
	}
}
