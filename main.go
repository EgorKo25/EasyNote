package main

import (
	"context"

	"github.com/EgorKo25/EasyNote/src/background"
	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database/postgresql"
	"github.com/EgorKo25/EasyNote/src/logger"
	"github.com/EgorKo25/EasyNote/src/scheduler"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		panic(err)
	}

	if err = logger.Initialize(cfg.AppConfig, cfg.Logger); err != nil {
		panic(err)
	}

	log := logger.Get("main")

	defer func() {
		err = log.Sync()
		if err != nil {
			panic(err)
		}
	}()

	log.With(logger.Any("config", cfg)).Info("the application configuration is initialized")

	database, err := postgresql.NewDatabase(cfg.Database)
	if err != nil {
		log.Error("cannot connect to database", err)
		panic(err)
	}

	ctx := context.Background()

	if err = database.Migrate(ctx); err != nil {
		log.Error("cannot migrate database", err)
		panic(err)
	}

	runner := background.NewRunner()

	runner.Run(
		ctx,
		scheduler.New(database, cfg.PollInterval),
	)
}
