// Package config is a configurator for  all app's services and modules
//
//nolint:lll,tagalign // The linter was disabled because it had a conflict with the tagalign linter. Tagalign linter is more important in this file.
package config

import (
	"fmt"
	"os"
	"time"

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
	AppConfig      `validate:"required" yaml:"app"`
	LoggerConfig   `validate:"required" yaml:"logger"`
	DatabaseConfig `validate:"required" yaml:"database"`
}

type SchedulerConfig struct {
	CacheWindow     time.Duration `default:"24h" validate:"required,gt=0"  yaml:"cache_window"`
	RefreshInterval time.Duration `default:"5m"  validate:"required,gt=0"  yaml:"refresh_interval"`
	LockTimeout     time.Duration `default:"1m"  validate:"required,gt=0"  yaml:"lock_timeout"`
	MaxAttempts     int           `default:"5"   validate:"required,min=1" yaml:"max_attempts"`
}

type AppConfig struct {
	Name        string `default:"easy-note" validate:"required"                                       yaml:"name"`
	Environment string `default:"local"     validate:"required,oneof=local dev test stage production" yaml:"environment"`
}

type LoggerConfig struct {
	Cores []LoggerCoreConfig `validate:"required,min=1,dive" yaml:"cores"`
}

type LoggerCoreConfig struct {
	Name       string `validate:"required"                                                yaml:"name"`
	Level      string `validate:"required,oneof=debug info warn error dpanic panic fatal" yaml:"level"       default:"info"`
	Encoding   string `validate:"required,oneof=json console"                             yaml:"encoding"    default:"json"`
	Output     string `validate:"required,oneof=stdout stderr file"                       yaml:"output"      default:"stdout"`
	Path       string `validate:"required_if=Output file"                                 yaml:"path"`
	TimeFormat string `validate:"required,oneof=utc local"                                yaml:"time_format" default:"utc"`
}
type DatabaseConfig struct {
	Host     string `default:"localhost"     validate:"required"                                             yaml:"host"`
	Port     int    `default:"5432"          validate:"required,min=1,max=65535"                             yaml:"port"`
	User     string `default:"enote"         validate:"required"                                             yaml:"user"`
	Password string `default:"enotePassword" validate:"required"                                             yaml:"password"`
	Name     string `default:"easy_note"     validate:"required"                                             yaml:"name"`
	SSLMode  string `default:"disable"       validate:"required,oneof=disable require verify-ca verify-full" yaml:"ssl_mode"`
}
