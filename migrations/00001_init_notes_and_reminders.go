package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upInitNotesAndReminders, downInitNotesAndReminders)
}

func upInitNotesAndReminders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
	
	CREATE TABLE notes (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	
		telegram_chat_id BIGINT NOT NULL,
	
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
	
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	
	CREATE TABLE reminders (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	
		note_id BIGINT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		remind_at TIMESTAMPTZ NOT NULL,
	
		status TEXT NOT NULL DEFAULT 'pending'
			CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'canceled')),
	
		attempts INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
		sent_at TIMESTAMPTZ,
	
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

	CREATE INDEX reminders_due_idx
		ON reminders (status, remind_at)
		WHERE status IN ('pending', 'failed');
`)
	if err != nil {
		return fmt.Errorf("apply init notes and reminders migration: %w", err)
	}

	return nil
}

func downInitNotesAndReminders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS reminders;
		DROP TABLE IF EXISTS notes;
`)
	if err != nil {
		return fmt.Errorf("rollback init notes and reminders migration: %w", err)
	}

	return nil
}
