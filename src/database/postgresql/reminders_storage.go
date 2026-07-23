package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/EgorKo25/EasyNote/src/database"
)

var errUnsupportedReminderAcknowledgmentStatus = errors.New("unsupported reminder acknowledgment status")

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

func (d *Database) FindRemindersByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]database.ReminderView, error) {
	query := `
		SELECT r.id, r.rule_id, n.title, n.description, r.remind_at, r.status, r.created_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.telegram_chat_id = $1
			AND r.status = 'new'
		ORDER BY r.remind_at, r.id
		LIMIT $2 OFFSET $3
	`

	rows, err := d.pool.Query(ctx, query, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find reminders by chat: %w", err)
	}

	reminders, err := pgx.CollectRows(rows, pgx.RowToStructByName[database.ReminderView])
	if err != nil {
		return nil, fmt.Errorf("collect reminders by chat: %w", err)
	}

	return reminders, nil
}

func (d *Database) FindReminderHistoryByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]database.ReminderView, error) {
	query := `
		SELECT r.id, r.rule_id, n.title, n.description, r.remind_at, r.status, r.created_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.telegram_chat_id = $1
			AND r.status <> 'new'
		ORDER BY r.remind_at DESC, r.id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := d.pool.Query(ctx, query, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find reminder history by chat: %w", err)
	}

	reminders, err := pgx.CollectRows(rows, pgx.RowToStructByName[database.ReminderView])
	if err != nil {
		return nil, fmt.Errorf("collect reminder history by chat: %w", err)
	}

	return reminders, nil
}

func (d *Database) ReminderByID(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
) (database.ReminderView, error) {
	query := `
		SELECT r.id, r.rule_id, n.title, n.description, r.remind_at, r.status, r.created_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.telegram_chat_id = $1
			AND r.id = $2
			AND r.status = 'new'
	`

	var reminder database.ReminderView
	if err := d.pool.QueryRow(ctx, query, telegramChatID, reminderID).Scan(
		&reminder.ID,
		&reminder.RuleID,
		&reminder.Title,
		&reminder.Description,
		&reminder.RemindAt,
		&reminder.Status,
		&reminder.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.ReminderView{}, fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
		}

		return database.ReminderView{}, fmt.Errorf("load reminder by id: %w", err)
	}

	return reminder, nil
}

func (d *Database) UpdateReminder(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
	title string,
	description string,
	remindAt time.Time,
) (database.ReminderView, error) {
	query := `
		WITH selected_reminder AS (
			SELECT r.id, r.note_id
			FROM reminders r
			JOIN notes n ON n.id = r.note_id
			WHERE n.telegram_chat_id = $1
				AND r.id = $2
				AND r.status = 'new'
			FOR UPDATE
		),
		updated_note AS (
			UPDATE notes n
			SET title = $3,
				description = $4
			FROM selected_reminder sr
			WHERE n.id = sr.note_id
			RETURNING n.id, n.title, n.description
		),
		updated_reminder AS (
			UPDATE reminders r
			SET remind_at = $5,
				last_sent_at = NULL,
				next_send_at = NULL,
				intrusive_until = NULL,
				telegram_message_id = NULL
			FROM selected_reminder sr
			WHERE r.id = sr.id
			RETURNING r.id, r.rule_id, r.note_id, r.remind_at, r.status, r.created_at
		)
		SELECT r.id, r.rule_id, n.title, n.description, r.remind_at, r.status, r.created_at
		FROM updated_reminder r
		JOIN updated_note n ON n.id = r.note_id
	`

	var reminder database.ReminderView
	if err := d.pool.QueryRow(ctx, query, telegramChatID, reminderID, title, description, remindAt).Scan(
		&reminder.ID,
		&reminder.RuleID,
		&reminder.Title,
		&reminder.Description,
		&reminder.RemindAt,
		&reminder.Status,
		&reminder.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.ReminderView{}, fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
		}

		return database.ReminderView{}, fmt.Errorf("update reminder: %w", err)
	}

	return reminder, nil
}

func (d *Database) ReminderStatsByChat(ctx context.Context, telegramChatID int64) (database.ReminderStats, error) {
	query := `
		SELECT
			count(*) FILTER (WHERE r.status = 'completed') AS completed,
			count(*) FILTER (WHERE r.status = 'new') AS pending,
			count(*) FILTER (WHERE r.status IN ('canceled', 'skipped', 'failed')) AS skipped
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.telegram_chat_id = $1
	`

	var stats database.ReminderStats

	err := d.pool.QueryRow(ctx, query, telegramChatID).Scan(
		&stats.Completed,
		&stats.Pending,
		&stats.Skipped,
	)
	if err != nil {
		return database.ReminderStats{}, fmt.Errorf("load reminder stats by chat: %w", err)
	}

	return stats, nil
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
		storage := newPlannerTransactionStorage(transaction)
		if err = storage.UpdateRuleStatus(ctx, *ruleID, database.ReminderRuleStatusCanceled); err != nil {
			return fmt.Errorf("cancel reminder rule: %w", err)
		}
	}

	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancel reminder transaction: %w", err)
	}

	return nil
}

func (d *Database) FindRemindersReadyToSend(
	ctx context.Context,
	now time.Time,
	limit int,
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
			JOIN notes n ON n.id = r.note_id
			WHERE r.remind_at <= $1
				AND r.status = 'new'
				AND (
					r.next_send_at IS NULL
					OR r.next_send_at <= $1
					OR r.intrusive_until <= $1
				)
			ORDER BY r.remind_at, r.id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		SELECT
			r.id,
			r.note_id,
			r.rule_id,
			n.telegram_chat_id,
			n.title,
			n.description,
			r.remind_at,
			COALESCE(s.intrusive_enabled, false),
			(COALESCE(s.intrusive_interval_minutes, 30) * INTERVAL '1 minute') AS intrusive_interval,
			COALESCE(s.timezone, 'Europe/Moscow'),
			r.telegram_message_id
		FROM reminders r
		JOIN due d ON d.id = r.id
		JOIN notes n ON n.id = r.note_id
		LEFT JOIN user_settings s ON s.telegram_chat_id = n.telegram_chat_id
		ORDER BY r.remind_at, r.id
	`

	rows, err := transaction.Query(ctx, query, now, limit)
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
			&reminder.IntrusiveEnabled,
			&reminder.IntrusiveInterval,
			&reminder.Timezone,
			&reminder.TelegramMessageID,
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

func (d *Database) MarkReminderDelivered(ctx context.Context, delivery database.ReminderDelivery) error {
	query := `
		UPDATE reminders
		SET last_sent_at = now(),
			next_send_at = $2,
			intrusive_until = $3,
			telegram_message_id = $4
		WHERE id = $1
			AND status = 'new'
	`

	if affectedRows, err := d.pool.Exec(
		ctx,
		query,
		delivery.ReminderID,
		delivery.NextSendAt,
		delivery.IntrusiveUntil,
		delivery.MessageID,
	); err != nil {
		return fmt.Errorf("mark reminder delivered: %w", err)
	} else if affectedRows.RowsAffected() == 0 {
		return fmt.Errorf("mark reminder delivered: %w", database.ErrReminderNotFound)
	}

	return nil
}

func (d *Database) AcknowledgeReminder(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
	status database.ReminderStatus,
) error {
	if status != database.ReminderStatusCompleted && status != database.ReminderStatusCanceled {
		return fmt.Errorf("%w: %s", errUnsupportedReminderAcknowledgmentStatus, status)
	}

	query := `
		UPDATE reminders r
		SET status = $3,
			sent_at = now(),
			next_send_at = NULL
		FROM notes n
		WHERE n.id = r.note_id
			AND n.telegram_chat_id = $1
			AND r.id = $2
			AND r.status IN ('new', 'sent')
	`

	if affectedRows, err := d.pool.Exec(ctx, query, telegramChatID, reminderID, status); err != nil {
		return fmt.Errorf("acknowledge reminder: %w", err)
	} else if affectedRows.RowsAffected() == 0 {
		return fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
	}

	return nil
}
