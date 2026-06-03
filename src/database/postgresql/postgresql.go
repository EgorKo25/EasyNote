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

var ErrUnsupportedReminderCompletionStatus = errors.New("unsupported reminder completion status")

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

	reminder, err := insertReminder(ctx, transaction, noteID, &ruleID, request.FirstRemindAt)
	if err != nil {
		return database.ReminderView{}, err
	}

	for _, remindDate := range request.CustomDates {
		generatedReminderID := any(nil)
		if sameDate(remindDate, request.FirstRemindAt) {
			generatedReminderID = reminder.ID
		}

		if _, err = transaction.Exec(
			ctx,
			`INSERT INTO reminder_custom_dates (rule_id, remind_date, generated_reminder_id)
			VALUES ($1, $2, $3)`,
			ruleID,
			remindDate,
			generatedReminderID,
		); err != nil {
			return database.ReminderView{}, fmt.Errorf("create custom reminder date: %w", err)
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
	query := `
		UPDATE reminders r
		SET status = 'canceled'
		FROM notes n
		WHERE n.id = r.note_id
			AND n.telegram_chat_id = $1
			AND r.id = $2
			AND r.status = 'new'
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

func (d *Database) MarkReminderFailed(ctx context.Context, reminderID int64, _ int) error {
	query := `
		UPDATE reminders
		SET attempts = attempts + 1,
			status = 'failed'
		WHERE id = $1
	`

	if _, err := d.pool.Exec(ctx, query, reminderID); err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}

	return nil
}

func (d *Database) CompleteReminder(ctx context.Context, request database.ReminderCompletionRequest) error {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete reminder transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "complete reminder")

	reminder, err := loadScheduledReminderForUpdate(ctx, transaction, request.ReminderID)
	if err != nil {
		return err
	}

	if err = markReminderCompleted(ctx, transaction, request); err != nil {
		return err
	}

	if reminder.RuleID == nil {
		return commitCompleteReminder(ctx, transaction)
	}

	if err = createNextRecurringReminder(ctx, transaction, request, reminder); err != nil {
		return err
	}

	return commitCompleteReminder(ctx, transaction)
}

func markReminderCompleted(
	ctx context.Context,
	transaction pgx.Tx,
	request database.ReminderCompletionRequest,
) error {
	switch request.Status {
	case database.ReminderCompletionStatusSent:
		return markReminderSent(ctx, transaction, request.ReminderID, request.SentAt)
	case database.ReminderCompletionStatusFailed:
		return markReminderFailed(ctx, transaction, request.ReminderID)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedReminderCompletionStatus, request.Status)
	}
}

func createNextRecurringReminder(
	ctx context.Context,
	transaction pgx.Tx,
	request database.ReminderCompletionRequest,
	reminder database.ScheduledReminder,
) error {
	rule, err := loadReminderRuleForUpdate(ctx, transaction, *reminder.RuleID)
	if err != nil {
		return err
	}

	var customDates []database.CustomReminderDate
	if rule.Type == database.ReminderRuleTypeCustomDates {
		customDates, err = loadCustomReminderDatesForUpdate(ctx, transaction, rule.ID)
		if err != nil {
			return err
		}
	}

	next, err := request.Next(database.ReminderCompletionInput{
		Reminder:    reminder,
		Rule:        &rule,
		CustomDates: customDates,
	})
	if err != nil {
		return fmt.Errorf("calculate next reminder: %w", err)
	}

	if next.RemindAt != nil {
		if err = insertNextRecurringReminder(ctx, transaction, reminder, rule, *next.RemindAt); err != nil {
			return err
		}
	}

	if next.CompleteRule {
		if err = completeReminderRule(ctx, transaction, rule.ID); err != nil {
			return err
		}
	}

	return nil
}

func insertNextRecurringReminder(
	ctx context.Context,
	transaction pgx.Tx,
	reminder database.ScheduledReminder,
	rule database.ReminderRule,
	remindAt time.Time,
) error {
	nextReminder, err := insertReminder(ctx, transaction, reminder.NoteID, reminder.RuleID, remindAt)
	if err != nil {
		return err
	}

	if rule.Type != database.ReminderRuleTypeCustomDates {
		return nil
	}

	if err = markCustomDateGenerated(ctx, transaction, rule.ID, remindAt, nextReminder.ID); err != nil {
		return err
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

func loadScheduledReminderForUpdate(
	ctx context.Context,
	transaction pgx.Tx,
	reminderID int64,
) (database.ScheduledReminder, error) {
	query := `
		SELECT r.id, r.note_id, r.rule_id, n.telegram_chat_id, n.title, n.description, r.remind_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE r.id = $1
		FOR UPDATE OF r
	`

	var reminder database.ScheduledReminder
	if err := transaction.QueryRow(ctx, query, reminderID).Scan(
		&reminder.ID,
		&reminder.NoteID,
		&reminder.RuleID,
		&reminder.TelegramChatID,
		&reminder.Title,
		&reminder.Description,
		&reminder.RemindAt,
	); err != nil {
		return database.ScheduledReminder{}, fmt.Errorf("load reminder for completion: %w", err)
	}

	return reminder, nil
}

func loadReminderRuleForUpdate(ctx context.Context, transaction pgx.Tx, ruleID int64) (database.ReminderRule, error) {
	query := `
		SELECT id, type, interval_days, weekdays_mask, status, created_at, updated_at
		FROM reminder_rules
		WHERE id = $1
		FOR UPDATE
	`

	var rule database.ReminderRule
	if err := transaction.QueryRow(ctx, query, ruleID).Scan(
		&rule.ID,
		&rule.Type,
		&rule.IntervalDays,
		&rule.WeekdaysMask,
		&rule.Status,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	); err != nil {
		return database.ReminderRule{}, fmt.Errorf("load reminder rule: %w", err)
	}

	return rule, nil
}

func loadCustomReminderDatesForUpdate(
	ctx context.Context,
	transaction pgx.Tx,
	ruleID int64,
) ([]database.CustomReminderDate, error) {
	query := `
		SELECT rule_id, remind_date, generated_reminder_id, created_at
		FROM reminder_custom_dates
		WHERE rule_id = $1
		ORDER BY remind_date
		FOR UPDATE
	`

	rows, err := transaction.Query(ctx, query, ruleID)
	if err != nil {
		return nil, fmt.Errorf("load custom reminder dates: %w", err)
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

func insertReminder(
	ctx context.Context,
	transaction pgx.Tx,
	noteID int64,
	ruleID *int64,
	remindAt time.Time,
) (database.Reminder, error) {
	query := `
		INSERT INTO reminders (note_id, rule_id, remind_at)
		VALUES ($1, $2, $3)
		RETURNING id, note_id, rule_id, remind_at, status, attempts, sent_at, created_at
	`

	var reminder database.Reminder
	if err := transaction.QueryRow(ctx, query, noteID, ruleID, remindAt).Scan(
		&reminder.ID,
		&reminder.NoteID,
		&reminder.RuleID,
		&reminder.RemindAt,
		&reminder.Status,
		&reminder.Attempts,
		&reminder.SentAt,
		&reminder.CreatedAt,
	); err != nil {
		return database.Reminder{}, fmt.Errorf("insert reminder: %w", err)
	}

	return reminder, nil
}

func markReminderSent(ctx context.Context, transaction pgx.Tx, reminderID int64, sentAt *time.Time) error {
	query := `
		UPDATE reminders
		SET status = 'sent',
			sent_at = $2
		WHERE id = $1
	`

	if _, err := transaction.Exec(ctx, query, reminderID, sentAt); err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}

	return nil
}

func markReminderFailed(ctx context.Context, transaction pgx.Tx, reminderID int64) error {
	query := `
		UPDATE reminders
		SET attempts = attempts + 1,
			status = 'failed'
		WHERE id = $1
	`

	if _, err := transaction.Exec(ctx, query, reminderID); err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}

	return nil
}

func markCustomDateGenerated(
	ctx context.Context,
	transaction pgx.Tx,
	ruleID int64,
	remindAt time.Time,
	reminderID int64,
) error {
	query := `
		UPDATE reminder_custom_dates
		SET generated_reminder_id = $3
		WHERE rule_id = $1
			AND remind_date = $2::date
	`

	if _, err := transaction.Exec(ctx, query, ruleID, remindAt, reminderID); err != nil {
		return fmt.Errorf("mark custom reminder date generated: %w", err)
	}

	return nil
}

func completeReminderRule(ctx context.Context, transaction pgx.Tx, ruleID int64) error {
	query := `
		UPDATE reminder_rules
		SET status = 'completed',
			updated_at = now()
		WHERE id = $1
	`

	if _, err := transaction.Exec(ctx, query, ruleID); err != nil {
		return fmt.Errorf("complete reminder rule: %w", err)
	}

	return nil
}

func commitCompleteReminder(ctx context.Context, transaction pgx.Tx) error {
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete reminder transaction: %w", err)
	}

	return nil
}

func rollbackTransaction(ctx context.Context, transaction pgx.Tx, operation string) {
	if rollbackErr := transaction.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
		logger.Get("database/postgresql").Error("cannot rollback "+operation+" transaction", rollbackErr)
	}
}

func sameDate(left time.Time, right time.Time) bool {
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
}
