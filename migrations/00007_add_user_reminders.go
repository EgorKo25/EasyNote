package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddUserReminders, downAddUserReminders)
}

func upAddUserReminders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
	CREATE TABLE bot_users (
		telegram_chat_id BIGINT PRIMARY KEY,
		username TEXT NOT NULL CHECK (length(trim(username)) > 0),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (lower(username))
	);

	CREATE TABLE reminder_permissions (
		owner_chat_id BIGINT NOT NULL,
		sender_chat_id BIGINT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (owner_chat_id, sender_chat_id)
	);

	ALTER TABLE user_settings
		ADD COLUMN incoming_reminder_requests_enabled BOOLEAN NOT NULL DEFAULT false;

	ALTER TABLE notes
		ADD COLUMN sender_chat_id BIGINT,
		ADD COLUMN recipient_username TEXT;

	UPDATE notes
	SET sender_chat_id = telegram_chat_id;

	ALTER TABLE notes
		ALTER COLUMN sender_chat_id SET NOT NULL;

	ALTER TABLE reminders
		DROP CONSTRAINT IF EXISTS reminders_status_check;

	ALTER TABLE reminders
		ADD CONSTRAINT reminders_status_check CHECK (
			status IN (
				'new',
				'sent',
				'failed',
				'canceled',
				'completed',
				'skipped',
				'pending_confirmation',
				'declined'
			)
		);

	CREATE INDEX notes_sender_created_idx
		ON notes (sender_chat_id, created_at DESC, id DESC);

	CREATE INDEX reminders_pending_confirmation_idx
		ON reminders (status, id)
		WHERE status = 'pending_confirmation';
`)
	if err != nil {
		return fmt.Errorf("apply user reminders migration: %w", err)
	}

	return nil
}

func downAddUserReminders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
	DROP INDEX IF EXISTS reminders_pending_confirmation_idx;
	DROP INDEX IF EXISTS notes_sender_created_idx;

	UPDATE reminders
	SET status = 'canceled'
	WHERE status IN ('pending_confirmation', 'declined');

	ALTER TABLE reminders
		DROP CONSTRAINT IF EXISTS reminders_status_check;

	ALTER TABLE reminders
		ADD CONSTRAINT reminders_status_check CHECK (
			status IN ('new', 'sent', 'failed', 'canceled', 'completed', 'skipped')
		);

	ALTER TABLE notes
		DROP COLUMN IF EXISTS recipient_username,
		DROP COLUMN IF EXISTS sender_chat_id;

	ALTER TABLE user_settings
		DROP COLUMN IF EXISTS incoming_reminder_requests_enabled;

	DROP TABLE IF EXISTS reminder_permissions;
	DROP TABLE IF EXISTS bot_users;
`)
	if err != nil {
		return fmt.Errorf("rollback user reminders migration: %w", err)
	}

	return nil
}
