package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/EgorKo25/EasyNote/src/database"
)

func (d *Database) SaveReminder(ctx context.Context, reminder *database.Reminder) error {
	query := `
		INSERT INTO reminders (note_id, rule_id, remind_at, status, attempts, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	if reminder.Status == "" {
		reminder.Status = database.ReminderStatusNew
	}

	if err := d.pool.QueryRow(
		ctx,
		query,
		reminder.NoteID,
		reminder.RuleID,
		reminder.RemindAt,
		reminder.Status,
		reminder.Attempts,
		reminder.SentAt,
	).Scan(&reminder.ID, &reminder.CreatedAt); err != nil {
		return fmt.Errorf("save reminder: %w", err)
	}

	return nil
}

func (d *Database) CreateReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
) (database.ReminderView, error) {
	query := `
		WITH created_note AS (
			INSERT INTO notes (telegram_chat_id, title, description)
			VALUES ($1, $2, $3)
			RETURNING id
		),
		created_reminder AS (
			INSERT INTO reminders (note_id, remind_at)
			SELECT id, $4
			FROM created_note
			RETURNING id, remind_at, status, created_at
		)
		SELECT r.id, $2::text, $3::text, r.remind_at, r.status, r.created_at
		FROM created_reminder r
	`

	var reminder database.ReminderView
	if err := d.pool.QueryRow(ctx, query, telegramChatID, title, description, remindAt).Scan(
		&reminder.ID,
		&reminder.Title,
		&reminder.Description,
		&reminder.RemindAt,
		&reminder.Status,
		&reminder.CreatedAt,
	); err != nil {
		return database.ReminderView{}, fmt.Errorf("create reminder: %w", err)
	}

	return reminder, nil
}

func (d *Database) FindRemindersByChat(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error) {
	query := `
		SELECT r.id, r.rule_id, n.title, n.description, r.remind_at, r.status, r.created_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.telegram_chat_id = $1
			AND r.status = 'new'
		ORDER BY r.remind_at, r.id
	`

	rows, err := d.pool.Query(ctx, query, telegramChatID)
	if err != nil {
		return nil, fmt.Errorf("find reminders by chat: %w", err)
	}
	defer rows.Close()

	reminders := make([]database.ReminderView, 0)

	for rows.Next() {
		var reminder database.ReminderView
		if err = rows.Scan(
			&reminder.ID,
			&reminder.RuleID,
			&reminder.Title,
			&reminder.Description,
			&reminder.RemindAt,
			&reminder.Status,
			&reminder.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reminder by chat: %w", err)
		}

		reminders = append(reminders, reminder)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminders by chat: %w", err)
	}

	return reminders, nil
}

func (d *Database) CancelReminder(ctx context.Context, telegramChatID int64, reminderID int64) error {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin cancel reminder transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "cancel reminder")

	query := `
		UPDATE reminders r
		SET status = 'canceled'
		FROM notes n
		WHERE n.id = r.note_id
			AND n.telegram_chat_id = $1
			AND r.id = $2
			AND r.status = 'new'
		RETURNING r.rule_id
	`

	var ruleID *int64
	if err = transaction.QueryRow(ctx, query, telegramChatID, reminderID).Scan(&ruleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
		}

		return fmt.Errorf("cancel reminder: %w", err)
	}

	if ruleID != nil {
		storage := newTransactionStorage(transaction)
		if err = storage.UpdateRuleStatus(ctx, *ruleID, database.ReminderRuleStatusCanceled); err != nil {
			return fmt.Errorf("cancel reminder rule: %w", err)
		}
	}

	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancel reminder transaction: %w", err)
	}

	return nil
}

func (d *Database) ActualReminders(
	ctx context.Context,
	from time.Time,
	until time.Time,
) ([]*database.ScheduledReminder, error) {
	query := `
		SELECT r.id, r.note_id, r.rule_id, n.telegram_chat_id, n.title, n.description, r.remind_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE r.remind_at >= $1
			AND r.remind_at <= $2
			AND r.status = 'new'
		ORDER BY r.remind_at, r.id
	`

	rows, err := d.pool.Query(ctx, query, from, until)
	if err != nil {
		return nil, fmt.Errorf("load actual reminders: %w", err)
	}
	defer rows.Close()

	return scanScheduledReminders(rows)
}

func (d *Database) FindRemindersReadyToSend(
	ctx context.Context,
	now time.Time,
	limit, maxAttempts int,
) ([]*database.ScheduledReminder, error) {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin find reminders ready to send transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "find reminders ready to send")

	query := `
		WITH due AS (
			SELECT r.id
			FROM reminders r
			WHERE r.remind_at <= $1
				AND r.status = 'new'
				AND r.attempts < $2
			ORDER BY r.remind_at, r.id
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		SELECT r.id, r.note_id, r.rule_id, n.telegram_chat_id, n.title, n.description, r.remind_at
		FROM reminders r
		JOIN due d ON d.id = r.id
		JOIN notes n ON n.id = r.note_id
		ORDER BY r.remind_at, r.id
	`

	rows, err := transaction.Query(ctx, query, now, maxAttempts, limit)
	if err != nil {
		return nil, fmt.Errorf("query reminders ready to send: %w", err)
	}
	defer rows.Close()

	reminders, err := scanScheduledReminders(rows)
	if err != nil {
		return nil, err
	}

	if err = transaction.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit find reminders ready to send transaction: %w", err)
	}

	return reminders, nil
}

func scanScheduledReminders(rows pgx.Rows) ([]*database.ScheduledReminder, error) {
	reminders := make([]*database.ScheduledReminder, 0)

	for rows.Next() {
		var reminder database.ScheduledReminder
		if err := rows.Scan(
			&reminder.ID,
			&reminder.NoteID,
			&reminder.RuleID,
			&reminder.TelegramChatID,
			&reminder.Title,
			&reminder.Description,
			&reminder.RemindAt,
		); err != nil {
			return nil, fmt.Errorf("scan scheduled reminder: %w", err)
		}

		reminders = append(reminders, &reminder)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled reminders: %w", err)
	}

	return reminders, nil
}
