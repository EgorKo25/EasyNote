package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upRelaxIntrusiveIntervalAndDefaultTimezone, downRelaxIntrusiveIntervalAndDefaultTimezone)
}

func upRelaxIntrusiveIntervalAndDefaultTimezone(ctx context.Context, transaction *sql.Tx) error {
	query := `
		ALTER TABLE user_settings
			ALTER COLUMN timezone SET DEFAULT 'Europe/Moscow',
			DROP CONSTRAINT IF EXISTS user_settings_intrusive_interval_check,
			ADD CONSTRAINT user_settings_intrusive_interval_check CHECK (
				intrusive_interval_minutes BETWEEN 1 AND 120
			);

		UPDATE user_settings
		SET timezone = 'Europe/Moscow'
		WHERE timezone = 'Local';
	`

	if _, err := transaction.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("relax intrusive interval and default timezone: %w", err)
	}

	return nil
}

func downRelaxIntrusiveIntervalAndDefaultTimezone(ctx context.Context, transaction *sql.Tx) error {
	query := `
		UPDATE user_settings
		SET intrusive_interval_minutes = 30
		WHERE intrusive_interval_minutes < 30;

		ALTER TABLE user_settings
			ALTER COLUMN timezone SET DEFAULT 'Local',
			DROP CONSTRAINT IF EXISTS user_settings_intrusive_interval_check,
			ADD CONSTRAINT user_settings_intrusive_interval_check CHECK (
				intrusive_interval_minutes BETWEEN 30 AND 1440
			);
	`

	if _, err := transaction.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("restore intrusive interval and default timezone: %w", err)
	}

	return nil
}
