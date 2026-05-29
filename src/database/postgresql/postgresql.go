package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	_ "github.com/EgorKo25/EasyNote/migrations"
	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
)

type Database struct {
	pool *pgxpool.Pool
	cfg  config.Database
}

func NewDatabase(ctx context.Context, cfg config.Database) (*Database, error) {
	connectionString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("parse postgres pool config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping postgres pool: %w", err)
	}

	return &Database{pool: pool, cfg: cfg}, nil
}

func (d *Database) Migrate(ctx context.Context) error {
	if err := goose.SetDialect(d.cfg.MigrationDialect); err != nil {
		return fmt.Errorf("set goose dialect %q: %w", d.cfg.MigrationDialect, err)
	}

	sqlDB := stdlib.OpenDBFromPool(d.pool)

	err := goose.UpContext(ctx, sqlDB, ".")
	if closeErr := sqlDB.Close(); closeErr != nil {
		logger.Get("database/postgresql").Error("cannot close migration sql database", closeErr)
	}

	if err != nil {
		return fmt.Errorf("run database migrations: %w", err)
	}

	return nil
}

func (d *Database) SaveReminder(ctx context.Context, reminder *database.Reminder) error {
	query := `
		INSERT INTO reminders (note_id, remind_at, status, attempts, sent_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	if reminder.Status == "" {
		reminder.Status = database.ReminderStatusPending
	}

	if err := d.pool.QueryRow(
		ctx,
		query,
		reminder.NoteID,
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
		SELECT r.id, n.title, n.description, r.remind_at, r.status, r.created_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.telegram_chat_id = $1
			AND r.status IN ('pending', 'processing', 'failed')
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
	query := `
		UPDATE reminders r
		SET status = 'canceled'
		FROM notes n
		WHERE n.id = r.note_id
			AND n.telegram_chat_id = $1
			AND r.id = $2
			AND r.status IN ('pending', 'processing', 'failed')
	`

	tag, err := d.pool.Exec(ctx, query, telegramChatID, reminderID)
	if err != nil {
		return fmt.Errorf("cancel reminder: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
	}

	return nil
}

func (d *Database) ActualReminders(
	ctx context.Context,
	from time.Time,
	until time.Time,
) ([]*database.ScheduledReminder, error) {
	query := `
		SELECT r.id, r.note_id, n.telegram_chat_id, n.title, n.description, r.remind_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE r.remind_at >= $1
			AND r.remind_at <= $2
			AND r.status IN ('pending', 'failed')
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

	defer func() {
		if rollbackErr := transaction.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			logger.Get("database/postgresql").
				Error("cannot rollback find reminders ready to send transaction", rollbackErr)
		}
	}()

	query := `
		WITH due AS (
			SELECT r.id
			FROM reminders r
			WHERE r.remind_at <= $1
				AND r.status IN ('pending', 'failed')
				AND r.attempts < $2
			ORDER BY r.remind_at, r.id
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		),
		locked AS (
			UPDATE reminders r
			SET status = 'processing'
			FROM due
			WHERE r.id = due.id
			RETURNING r.id, r.note_id, r.remind_at
		)
		SELECT l.id, l.note_id, n.telegram_chat_id, n.title, n.description, l.remind_at
		FROM locked l
		JOIN notes n ON n.id = l.note_id
		ORDER BY l.remind_at, l.id
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

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("close reminder rows ready to send: %w", err)
	}

	if err = transaction.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit find reminders ready to send transaction: %w", err)
	}

	return reminders, nil
}

func (d *Database) MarkReminderSent(ctx context.Context, reminderID int64, sentAt time.Time) error {
	query := `
		UPDATE reminders
		SET status = 'sent',
			sent_at = $2
		WHERE id = $1
	`

	if _, err := d.pool.Exec(ctx, query, reminderID, sentAt); err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}

	return nil
}

func (d *Database) MarkReminderFailed(ctx context.Context, reminderID int64, maxAttempts int) error {
	query := `
		UPDATE reminders
		SET attempts = attempts + 1,
			status = CASE
				WHEN attempts + 1 >= $2 THEN 'failed'
				ELSE 'pending'
			END
		WHERE id = $1
	`

	if _, err := d.pool.Exec(ctx, query, reminderID, maxAttempts); err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}

	return nil
}

func scanScheduledReminders(rows pgx.Rows) ([]*database.ScheduledReminder, error) {
	reminders := make([]*database.ScheduledReminder, 0)

	for rows.Next() {
		var reminder database.ScheduledReminder
		if err := rows.Scan(
			&reminder.ID,
			&reminder.NoteID,
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
