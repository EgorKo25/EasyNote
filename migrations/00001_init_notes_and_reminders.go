package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upInitNotesAndReminders, downInitNotesAndReminders)
}

func upInitNotesAndReminders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
	
	CREATE TABLE notes (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY
	
		telegram_user_id BIGINT NOT NULL,
		telegram_chat_id BIGINT NOT NULL,
	
		text TEXT NOT NULL CHECK (length(trim(text)) > 0),
	
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	);
	
	CREATE TABLE reminders (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	
		note_id UUID NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
	
		text TEXT NOT NULL CHECK (length(trim(text)) > 0),
		remind_at TIMESTAMPTZ NOT NULL,
	
		status TEXT NOT NULL DEFAULT 'pending'
			CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'canceled')),
	
		attempts INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
		last_error TEXT,
	
		locked_until TIMESTAMPTZ,
		sent_at TIMESTAMPTZ,
	
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
);
`)
	return err
}

func downInitNotesAndReminders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS reminders;
		DROP TABLE IF EXISTS notes;
`)
	return err
}
