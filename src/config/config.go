// Package config is a configurator for  all app's services and modules
//
//nolint:lll,tagalign // The linter was disabled because it had a conflict with the tagalign linter. Tagalign linter is more important in this file.
package config

import (
	"time"

	"github.com/EgorKo25/kovio-common/kconfig"
)

const (
	localConfigPath = "./config/config.yaml"
)

func New() (*Config, error) {
	return kconfig.Load[Config](localConfigPath)
}

type Config struct {
	kconfig.Config `yaml:",inline"`
	Scheduler      `yaml:"scheduler" validate:"required"`
	Telegram       `yaml:"telegram"  validate:"required"`
	Database       `yaml:"database"  validate:"required"`
}

type Scheduler struct {
	PollInterval         time.Duration `default:"1m"       validate:"required,gt=0"                       yaml:"poll_interval"`
	SendRetryInterval    time.Duration `default:"5s"       validate:"required,gt=0"                       yaml:"send_retry_interval"`
	SendRetryMaxInterval time.Duration `default:"30s"      validate:"required,gt=0"                       yaml:"send_retry_max_interval"`
	BatchLimit           int           `default:"50"       validate:"required,min=1"                      yaml:"batch_limit"`
	SendRetryCount       int           `default:"3"        validate:"required,min=1"                      yaml:"send_retry_count"`
	SendRetryBackoff     string        `default:"constant" validate:"required,oneof=constant exponential" yaml:"send_retry_backoff"`
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
