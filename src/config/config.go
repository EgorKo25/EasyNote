package config

import (
	"fmt"
	"os"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

const (
	localConfigPath = "./config/config.yaml"
)

func New() (*Config, error) {
	var cfg Config

	if err := defaults.Set(&cfg); err != nil {
		return nil, fmt.Errorf("setting Default Configuration Values: %w", err)
	}

	data, err := os.ReadFile(localConfigPath)
	if err != nil {
		return nil, fmt.Errorf("read configuration file %q: %w", localConfigPath, err)
	}

	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing the configuration file %q: %w", localConfigPath, err)
	}

	if err = validate(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg Config) error {
	v := validator.New(validator.WithRequiredStructEnabled())

	if err := v.Struct(cfg); err != nil {
		return fmt.Errorf("configuration validation: %w", err)
	}

	return nil
}

type Config struct {
	AppConfig      `yaml:"app" validate:"required"`
	LoggerConfig   `yaml:"logger" validate:"required"`
	DatabaseConfig `yaml:"database" validate:"required"`
}

type AppConfig struct {
	Name        string `yaml:"name" default:"easy-note" validate:"required"`
	Environment string `yaml:"environment" default:"local" validate:"required,oneof=local dev test stage production"`
}

type LoggerConfig struct {
	Cores []LoggerCoreConfig `yaml:"cores" validate:"required,min=1,dive"`
}

type LoggerCoreConfig struct {
	Name       string `yaml:"name" validate:"required"`
	Level      string `yaml:"level" default:"info" validate:"required,oneof=debug info warn error dpanic panic fatal"`
	Encoding   string `yaml:"encoding" default:"json" validate:"required,oneof=json console"`
	Output     string `yaml:"output" default:"stdout" validate:"required,oneof=stdout stderr file"`
	Path       string `yaml:"path" validate:"required_if=Output file"`
	TimeFormat string `yaml:"time_format" default:"utc" validate:"required,oneof=utc local"`
}
type DatabaseConfig struct {
	Host     string `yaml:"host" default:"localhost" validate:"required"`
	Port     int    `yaml:"port" default:"5432" validate:"required,min=1,max=65535"`
	User     string `yaml:"user" default:"enote" validate:"required"`
	Password string `yaml:"password" default:"enotePassword" validate:"required"`
	Name     string `yaml:"name" default:"easy_note" validate:"required"`
	SSLMode  string `yaml:"ssl_mode" default:"disable" validate:"required,oneof=disable require verify-ca verify-full"`
}
