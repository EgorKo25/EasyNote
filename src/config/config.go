// Package config is a configurator for  all app's services and modules
//
//nolint:lll,tagalign // The linter was disabled because it had a conflict with the tagalign linter. Tagalign linter is more important in this file.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
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

	if err = env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parsing environment configuration: %w", err)
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
	AppConfig `validate:"required" yaml:"app"`
	Logger    `validate:"required" yaml:"logger"`
	Scheduler `validate:"required" yaml:"scheduler"`
	Telegram  `validate:"required" yaml:"telegram"`
	Database  `validate:"required" yaml:"database"`
}

type Scheduler struct {
	CacheWindow  time.Duration `default:"24h" validate:"required,gt=0"  yaml:"cache_window"`
	PollInterval time.Duration `default:"1m"  validate:"required,gt=0"  yaml:"poll_interval"`
	BatchLimit   int           `default:"50"  validate:"required,min=1" yaml:"batch_limit"`
	MaxAttempts  int           `default:"5"   validate:"required,min=1" yaml:"max_attempts"`
}

type AppConfig struct {
	Name        string `default:"easy-note" validate:"required"                                       yaml:"name"`
	Environment string `default:"local"     validate:"required,oneof=local dev test stage production" yaml:"environment" env:"APP_ENVIRONMENT"`
}

type Logger struct {
	Cores []LoggerCore `validate:"required,min=1,dive" yaml:"cores"`
}

type LoggerCore struct {
	Name       string `validate:"required"                                                yaml:"name"`
	Level      string `validate:"required,oneof=debug info warn error dpanic panic fatal" yaml:"level"       default:"info"`
	Encoding   string `validate:"required,oneof=json console"                             yaml:"encoding"    default:"json"`
	Output     string `validate:"required,oneof=stdout stderr file"                       yaml:"output"      default:"stdout"`
	Path       string `validate:"required_if=Output file"                                 yaml:"path"`
	TimeFormat string `validate:"required,oneof=utc local"                                yaml:"time_format" default:"utc"`
}

type Telegram struct {
	TokenEnv            string        `default:"TELEGRAM_BOT_TOKEN"       validate:"required"       yaml:"token_env"`
	APIBaseURL          string        `default:"https://api.telegram.org" validate:"required,url"   yaml:"api_base_url"`
	RequestTimeout      time.Duration `default:"10s"                      validate:"required,gt=0"  yaml:"request_timeout"`
	DebugMode           bool          `default:"false"                                              yaml:"debug_mode"`
	Warnings            bool          `default:"true"                                               yaml:"warnings"`
	HealthCheck         bool          `default:"false"                                              yaml:"health_check"`
	LongPollingBuffer   uint          `default:"100"                      validate:"required,min=1" yaml:"long_polling_buffer"`
	LongPollingInterval time.Duration `default:"0s"                       validate:"min=0"          yaml:"long_polling_interval"`
	LongPollingRetry    time.Duration `default:"8s"                       validate:"required,gt=0"  yaml:"long_polling_retry"`
	ParseMode           string        `                                                             yaml:"parse_mode"`
	DisableNotification bool          `default:"false"                                              yaml:"disable_notification"`
	ProtectContent      bool          `default:"false"                                              yaml:"protect_content"`
}

type Database struct {
	Host              string        `default:"localhost"     validate:"required"                                             yaml:"host"                env:"DATABASE_HOST"`
	Port              int           `default:"5432"          validate:"required,min=1,max=65535"                             yaml:"port"                env:"DATABASE_PORT"`
	User              string        `default:"enote"         validate:"required"                                             yaml:"user"                env:"DATABASE_USER"`
	Password          string        `default:"enotePassword" validate:"required"                                             yaml:"password"            env:"DATABASE_PASSWORD"`
	Name              string        `default:"easy_note"     validate:"required"                                             yaml:"name"                env:"DATABASE_NAME"`
	SSLMode           string        `default:"disable"       validate:"required,oneof=disable require verify-ca verify-full" yaml:"ssl_mode"            env:"DATABASE_SSL_MODE"`
	MigrationDialect  string        `default:"postgres"      validate:"required"                                             yaml:"migration_dialect"`
	MaxConns          int32         `default:"4"             validate:"required,min=1"                                       yaml:"max_conns"`
	MinConns          int32         `default:"0"             validate:"min=0"                                                yaml:"min_conns"`
	MaxConnLifetime   time.Duration `default:"1h"            validate:"required,gt=0"                                        yaml:"max_conn_lifetime"`
	MaxConnIdleTime   time.Duration `default:"30m"           validate:"required,gt=0"                                        yaml:"max_conn_idle_time"`
	HealthCheckPeriod time.Duration `default:"1m"            validate:"required,gt=0"                                        yaml:"health_check_period"`
}
