package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddHolidaysAndShoppingLists, downAddHolidaysAndShoppingLists)
}

func upAddHolidaysAndShoppingLists(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
	CREATE TABLE holidays (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		telegram_chat_id BIGINT NOT NULL,
		holiday_name TEXT NOT NULL CHECK (length(trim(holiday_name)) > 0),
		recipient_name TEXT NOT NULL CHECK (length(trim(recipient_name)) > 0),
		holiday_date DATE NOT NULL,
		remind_at TIMESTAMPTZ NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		last_sent_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);

	ALTER TABLE holidays
		ADD CONSTRAINT holidays_status_check CHECK (status IN ('active', 'canceled'));

	CREATE INDEX holidays_chat_created_idx
		ON holidays (telegram_chat_id, created_at DESC, id DESC);

	CREATE INDEX holidays_due_idx
		ON holidays (status, remind_at)
		WHERE status = 'active';

	CREATE TABLE shopping_lists (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		telegram_chat_id BIGINT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);

	CREATE INDEX shopping_lists_chat_created_idx
		ON shopping_lists (telegram_chat_id, created_at DESC, id DESC);

	CREATE TABLE shopping_list_items (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		list_id BIGINT NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
		text TEXT NOT NULL CHECK (length(trim(text)) > 0),
		done BOOLEAN NOT NULL DEFAULT false,
		position INT NOT NULL CHECK (position > 0),
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (list_id, position)
	);
`)
	if err != nil {
		return fmt.Errorf("apply holidays and shopping lists migration: %w", err)
	}

	return nil
}

func downAddHolidaysAndShoppingLists(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS shopping_list_items;
		DROP TABLE IF EXISTS shopping_lists;
		DROP TABLE IF EXISTS holidays;
`)
	if err != nil {
		return fmt.Errorf("rollback holidays and shopping lists migration: %w", err)
	}

	return nil
}
