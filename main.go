package main

import (
	"context"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database/postgresql"
	"github.com/EgorKo25/EasyNote/src/planner"
	"github.com/EgorKo25/EasyNote/src/scheduler"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/telegram"
	"github.com/EgorKo25/kovio-common/background"
	"github.com/EgorKo25/kovio-common/klogger"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		panic(err)
	}

	if err = klogger.Initialize(
		klogger.AppConfig{
			Name:        cfg.App.Name,
			Environment: cfg.App.Environment,
		},
		cfg.Logger,
	); err != nil {
		panic(err)
	}

	log := klogger.Get("main")

	defer func() { _ = log.Sync() }()

	log.With(klogger.Any("config", cfg)).Info("the application configuration is initialized")

	ctx := context.Background()

	database, err := postgresql.NewDatabase(ctx, cfg.Database)
	if err != nil {
		log.Error("cannot connect to database", err)
		panic(err)
	}

	if err = database.Migrate(ctx); err != nil {
		log.Error("cannot migrate database", err)
		panic(err)
	}

	telegramSender, err := telegram.NewSender(ctx, cfg.Telegram)
	if err != nil {
		log.Error("cannot initialize telegram sender", err)
		panic(err)
	}

	reminderPlanner := planner.New(database)
	reminderService := service.NewReminderService(database, reminderPlanner)
	reminderRunner := scheduler.NewRunner(database, telegramSender, reminderPlanner, cfg.Scheduler)
	reminderBot := telegram.NewReminderBot(telegramSender, reminderService, database, cfg.Telegram)

	runner := background.NewRunner()

	runner.Run(
		ctx,
		reminderRunner,
		reminderBot,
	)

	if err = runner.Wait(); err != nil {
		log.Error("background process failed", err)
		panic(err)
	}
}
