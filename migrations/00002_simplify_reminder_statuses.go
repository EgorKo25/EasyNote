package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upSimplifyReminderStatuses, downSimplifyReminderStatuses)
}

func upSimplifyReminderStatuses(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS reminders_due_idx;

		ALTER TABLE reminders
			DROP CONSTRAINT IF EXISTS reminders_status_check;

		UPDATE reminders
		SET status = CASE
			WHEN status IN ('pending', 'processing') THEN 'new'
			WHEN status = 'failed' THEN 'failed'
			WHEN status = 'canceled' THEN 'canceled'
			ELSE status
		END;

		ALTER TABLE reminders
			ALTER COLUMN status SET DEFAULT 'new',
			ADD CONSTRAINT reminders_status_check CHECK (status IN ('new', 'sent', 'failed', 'canceled')),
			DROP COLUMN IF EXISTS processing_started_at;

		CREATE INDEX reminders_due_idx
			ON reminders (status, remind_at)
			WHERE status = 'new';
`)
	if err != nil {
		return fmt.Errorf("simplify reminder statuses: %w", err)
	}

	return nil
}

func downSimplifyReminderStatuses(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS reminders_due_idx;

		ALTER TABLE reminders
			DROP CONSTRAINT IF EXISTS reminders_status_check;

		UPDATE reminders
		SET status = CASE
			WHEN status = 'new' THEN 'pending'
			ELSE status
		END;

		ALTER TABLE reminders
			ALTER COLUMN status SET DEFAULT 'pending',
			ADD CONSTRAINT reminders_status_check
				CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'canceled'));

		CREATE INDEX reminders_due_idx
			ON reminders (status, remind_at)
			WHERE status IN ('pending', 'failed');
`)
	if err != nil {
		return fmt.Errorf("rollback simplified reminder statuses: %w", err)
	}

	return nil
}
