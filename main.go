package main

import (
	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/logger"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		panic(err)
	}

	log, err := logger.New(cfg.AppConfig, cfg.LoggerConfig)
	if err != nil {
		panic(err)
	}

	defer func() {
		err = log.Sync()
		if err != nil {
			panic(err)
		}
	}()

	log.With(logger.Any("config", cfg)).Info("the application configuration is initialized")
}
