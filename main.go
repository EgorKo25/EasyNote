package main

import (
	"context"

	"github.com/EgorKo25/EasyNote/src/background"
	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database/postgresql"
	"github.com/EgorKo25/EasyNote/src/logger"
	"github.com/EgorKo25/EasyNote/src/planner"
	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	httprouter "github.com/EgorKo25/EasyNote/src/transport/http/router"
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

	defer func() { _ = log.Sync() }()

	log.With(logger.Any("config", cfg)).Info("the application configuration is initialized")

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

	reminderPlanner := planner.New(database)
	reminderService := service.NewReminderService(database, reminderPlanner)
	httpServer := apphttp.NewServer(cfg.HTTP, httprouter.Create(reminderService))

	runner := background.NewRunner()

	runner.Run(
		ctx,
		httpServer,
	)

	if err = runner.Wait(); err != nil {
		log.Error("background process failed", err)
		panic(err)
	}
}
