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

	ctx := context.Background()

	runner := background.NewRunner()

	db, err := postgresql.NewDatabase(cfg.Database)
	if err != nil {
		log.Fatal("cannot connect to database", err)
	}

	if err = db.Migrate(); err != nil {
		log.Fatal("cannot migrate database", err)
	}

	runner.Run(ctx,
		scheduler.New(db, scheduler.WithPollInterval(cfg.PollInterval)),
	)

}
