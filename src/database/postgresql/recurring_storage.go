package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/EgorKo25/EasyNote/src/database"
)

var errCreateCustomDate = errors.New("custom reminder date was not created")

func (d *Database) CreateRecurringReminder(
	ctx context.Context,
	request database.RecurringReminderRequest,
) (database.ReminderView, error) {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("begin create recurring reminder transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "create recurring reminder")

	var noteID int64
	if err = transaction.QueryRow(
		ctx,
		`
		INSERT INTO
			notes (telegram_chat_id, title, description)
		VALUES ($1, $2, $3)
		RETURNING id
		`,
		request.TelegramChatID,
		request.Title,
		request.Description,
	).Scan(&noteID); err != nil {
		return database.ReminderView{}, fmt.Errorf("create recurring reminder note: %w", err)
	}

	var ruleID int64
	if err = transaction.QueryRow(
		ctx,
		`
		INSERT INTO
			reminder_rules (type, interval_days, weekdays_mask)
		VALUES ($1, $2, $3)
		RETURNING id
		`,
		request.Rule.Type,
		request.Rule.IntervalDays,
		request.Rule.WeekdaysMask,
	).Scan(&ruleID); err != nil {
		return database.ReminderView{}, fmt.Errorf("create reminder rule: %w", err)
	}

	reminder, err := newTransactionStorage(transaction).
		CreateNextOccurrence(ctx, noteID, ruleID, request.FirstRemindAt)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create reminder: %w", err)
	}

	for _, remindDate := range request.CustomDates {
		if affectedRows, execErr := transaction.Exec(
			ctx,
			`
			INSERT INTO reminder_custom_dates (rule_id, remind_date, generated_reminder_id)
			VALUES (
				$1,
				$2,
				CASE WHEN $2::date = $3::date THEN $4 ELSE NULL END
			)
			`,
			ruleID,
			remindDate,
			request.FirstRemindAt,
			reminder.ID,
		); execErr != nil {
			return database.ReminderView{}, fmt.Errorf("create custom reminder date: %w", execErr)
		} else if affectedRows.RowsAffected() == 0 {
			return database.ReminderView{}, errCreateCustomDate
		}
	}

	if err = transaction.Commit(ctx); err != nil {
		return database.ReminderView{}, fmt.Errorf("commit create recurring reminder transaction: %w", err)
	}

	return database.ReminderView{
		ID:          reminder.ID,
		RuleID:      &ruleID,
		Title:       request.Title,
		Description: request.Description,
		RemindAt:    reminder.RemindAt,
		Status:      reminder.Status,
		CreatedAt:   reminder.CreatedAt,
	}, nil
}
