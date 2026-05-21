package main

import (
	"easynote/src/config"
	"easynote/src/logger"
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
	log.Info("the application configuration is initialized")
}
