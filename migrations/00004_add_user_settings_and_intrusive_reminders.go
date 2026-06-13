package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddUserSettingsAndIntrusiveReminders, downAddUserSettingsAndIntrusiveReminders)
}

func upAddUserSettingsAndIntrusiveReminders(ctx context.Context, transaction *sql.Tx) error {
	query := `
		CREATE TABLE user_settings (
			telegram_chat_id BIGINT PRIMARY KEY,
			language TEXT NOT NULL DEFAULT 'ru',
			timezone TEXT NOT NULL DEFAULT 'Europe/Moscow',
			intrusive_enabled BOOLEAN NOT NULL DEFAULT false,
			intrusive_interval_minutes INT NOT NULL DEFAULT 30,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			CONSTRAINT user_settings_language_check CHECK (language IN ('ru')),
			CONSTRAINT user_settings_intrusive_interval_check CHECK (
				intrusive_interval_minutes BETWEEN 1 AND 120
			)
		);

		ALTER TABLE reminders
			DROP CONSTRAINT IF EXISTS reminders_status_check;

		ALTER TABLE reminders
			ADD CONSTRAINT reminders_status_check CHECK (
				status IN ('new', 'sent', 'failed', 'canceled', 'completed', 'skipped')
			),
			ADD COLUMN last_sent_at TIMESTAMPTZ,
			ADD COLUMN next_send_at TIMESTAMPTZ,
			ADD COLUMN intrusive_until TIMESTAMPTZ,
			ADD COLUMN telegram_message_id INT;

		CREATE INDEX reminders_intrusive_due_idx
			ON reminders (status, next_send_at)
			WHERE status = 'new';
	`

	if _, err := transaction.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("add user settings and intrusive reminders: %w", err)
	}

	return nil
}

func downAddUserSettingsAndIntrusiveReminders(ctx context.Context, transaction *sql.Tx) error {
	query := `
		DROP INDEX IF EXISTS reminders_intrusive_due_idx;

		ALTER TABLE reminders
			DROP COLUMN IF EXISTS telegram_message_id,
			DROP COLUMN IF EXISTS intrusive_until,
			DROP COLUMN IF EXISTS next_send_at,
			DROP COLUMN IF EXISTS last_sent_at,
			DROP CONSTRAINT IF EXISTS reminders_status_check;

		UPDATE reminders
		SET status = 'canceled'
		WHERE status IN ('completed', 'skipped');

		ALTER TABLE reminders
			ADD CONSTRAINT reminders_status_check CHECK (status IN ('new', 'sent', 'failed', 'canceled'));

		DROP TABLE IF EXISTS user_settings;
	`

	if _, err := transaction.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("rollback user settings and intrusive reminders: %w", err)
	}

	return nil
}
