package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddRecurringReminderRules, downAddRecurringReminderRules)
}

func upAddRecurringReminderRules(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE reminder_rules (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			type TEXT NOT NULL CHECK (type IN ('interval', 'weekdays', 'custom_dates')),
			interval_days INT CHECK (interval_days IS NULL OR interval_days > 0),
			weekdays_mask INT CHECK (weekdays_mask IS NULL OR weekdays_mask BETWEEN 1 AND 127),
			status TEXT NOT NULL DEFAULT 'active'
				CHECK (status IN ('active', 'canceled', 'completed')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		ALTER TABLE reminders
			ADD COLUMN rule_id BIGINT REFERENCES reminder_rules(id) ON DELETE SET NULL;

		CREATE INDEX reminders_rule_id_idx
			ON reminders (rule_id)
			WHERE rule_id IS NOT NULL;

		CREATE UNIQUE INDEX reminders_one_new_per_rule_idx
			ON reminders (rule_id)
			WHERE rule_id IS NOT NULL AND status = 'new';

		CREATE TABLE reminder_custom_dates (
			rule_id BIGINT NOT NULL REFERENCES reminder_rules(id) ON DELETE CASCADE,
			remind_date DATE NOT NULL,
			generated_reminder_id BIGINT REFERENCES reminders(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (rule_id, remind_date)
		);
`)
	if err != nil {
		return fmt.Errorf("add recurring reminder rules: %w", err)
	}

	return nil
}

func downAddRecurringReminderRules(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS reminder_custom_dates;
		DROP INDEX IF EXISTS reminders_one_new_per_rule_idx;
		DROP INDEX IF EXISTS reminders_rule_id_idx;
		ALTER TABLE reminders DROP COLUMN IF EXISTS rule_id;
		DROP TABLE IF EXISTS reminder_rules;
`)
	if err != nil {
		return fmt.Errorf("rollback recurring reminder rules: %w", err)
	}

	return nil
}
