package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/planner"
)

type TransactionStorager = planner.TransactionStorage

var (
	errMarkReminderStatus      = errors.New("reminder status was not updated")
	errMarkCustomDateGenerated = errors.New("custom reminder date was not marked as generated")
	errUpdateRuleStatus        = errors.New("reminder rule status was not updated")
)

func newTransactionStorage(tx pgx.Tx) TransactionStorager {
	return transactionStorage{tx}
}

type transactionStorage struct{ pgx.Tx }

func (tx transactionStorage) LoadOccurrenceForUpdate(
	ctx context.Context,
	reminderID int64,
) (database.Reminder, error) {
	query := `
		SELECT id, note_id, rule_id, remind_at, status, attempts, sent_at, created_at
		FROM reminders
		WHERE id = $1
		FOR UPDATE
	`

	var reminder database.Reminder
	if err := tx.QueryRow(ctx, query, reminderID).Scan(
		&reminder.ID,
		&reminder.NoteID,
		&reminder.RuleID,
		&reminder.RemindAt,
		&reminder.Status,
		&reminder.Attempts,
		&reminder.SentAt,
		&reminder.CreatedAt,
	); err != nil {
		return database.Reminder{}, fmt.Errorf("load occurrence for update: %w", err)
	}

	return reminder, nil
}

func (tx transactionStorage) LoadRuleForUpdate(ctx context.Context, ruleID int64) (database.ReminderRule, error) {
	query := `
		SELECT id, type, interval_days, weekdays_mask, status, created_at, updated_at
		FROM reminder_rules
		WHERE id = $1
		FOR UPDATE
	`

	var rule database.ReminderRule
	if err := tx.QueryRow(ctx, query, ruleID).Scan(
		&rule.ID,
		&rule.Type,
		&rule.IntervalDays,
		&rule.WeekdaysMask,
		&rule.Status,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	); err != nil {
		return database.ReminderRule{}, fmt.Errorf("load reminder rule for update: %w", err)
	}

	return rule, nil
}

func (tx transactionStorage) LoadCustomDatesForUpdate(
	ctx context.Context,
	ruleID int64,
) ([]database.CustomReminderDate, error) {
	query := `
		SELECT rule_id, remind_date, generated_reminder_id, created_at
		FROM reminder_custom_dates
		WHERE rule_id = $1
		ORDER BY remind_date
		FOR UPDATE
	`

	rows, err := tx.Query(ctx, query, ruleID)
	if err != nil {
		return nil, fmt.Errorf("load custom reminder dates for update: %w", err)
	}
	defer rows.Close()

	dates := make([]database.CustomReminderDate, 0)

	for rows.Next() {
		var date database.CustomReminderDate
		if err = rows.Scan(&date.RuleID, &date.RemindDate, &date.GeneratedReminderID, &date.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan custom reminder date: %w", err)
		}

		dates = append(dates, date)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom reminder dates: %w", err)
	}

	return dates, nil
}

func (tx transactionStorage) MarkOccurrence(
	ctx context.Context,
	reminderID int64,
	status database.ReminderStatus,
	sentAt *time.Time,
) error {
	query := `
		UPDATE reminders
		SET status = $2,
			sent_at = $3
		WHERE id = $1 AND status = 'new'
	`
	args := []any{reminderID, status, sentAt}

	if status == database.ReminderStatusFailed {
		query = `
			UPDATE reminders
			SET attempts = attempts + 1,
				status = $2
			WHERE id = $1 AND status = 'new'
	`
		args = []any{reminderID, status}
	}

	if affectedRows, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark occurrence %s: %w", status, err)
	} else if affectedRows.RowsAffected() == 0 {
		return errMarkReminderStatus
	}

	return nil
}

func (tx transactionStorage) CreateNextOccurrence(
	ctx context.Context,
	noteID int64,
	ruleID int64,
	remindAt time.Time,
) (database.Reminder, error) {
	query := `
		INSERT INTO reminders (note_id, rule_id, remind_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (rule_id) WHERE rule_id IS NOT NULL AND status = 'new'
		DO UPDATE SET rule_id = EXCLUDED.rule_id
		RETURNING id, note_id, rule_id, remind_at, status, attempts, sent_at, created_at
	`

	var reminder database.Reminder
	if err := tx.QueryRow(ctx, query, noteID, ruleID, remindAt).Scan(
		&reminder.ID,
		&reminder.NoteID,
		&reminder.RuleID,
		&reminder.RemindAt,
		&reminder.Status,
		&reminder.Attempts,
		&reminder.SentAt,
		&reminder.CreatedAt,
	); err != nil {
		return database.Reminder{}, fmt.Errorf("create next occurrence: %w", err)
	}

	return reminder, nil
}

func (tx transactionStorage) MarkCustomDateGenerated(
	ctx context.Context,
	ruleID int64,
	remindDate time.Time,
	reminderID int64,
) error {
	query := `
		UPDATE reminder_custom_dates
		SET generated_reminder_id = $3
		WHERE rule_id = $1
			AND remind_date = $2::date
	`

	if affectedRows, err := tx.Exec(ctx, query, ruleID, remindDate, reminderID); err != nil {
		return fmt.Errorf("mark custom reminder date generated: %w", err)
	} else if affectedRows.RowsAffected() == 0 {
		return errMarkCustomDateGenerated
	}

	return nil
}

func (tx transactionStorage) UpdateRuleStatus(
	ctx context.Context,
	ruleID int64,
	status database.ReminderRuleStatus,
) error {
	query := `
		UPDATE reminder_rules
		SET status = $2,
			updated_at = now()
		WHERE id = $1
	`

	if affectedRows, err := tx.Exec(ctx, query, ruleID, status); err != nil {
		return fmt.Errorf("update reminder rule status: %w", err)
	} else if affectedRows.RowsAffected() == 0 {
		return errUpdateRuleStatus
	}

	return nil
}
