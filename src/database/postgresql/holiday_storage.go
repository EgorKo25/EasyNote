package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/EgorKo25/EasyNote/src/database"
)

func (d *Database) CreateHoliday(
	ctx context.Context,
	request database.HolidayRequest,
) (database.HolidayView, error) {
	query := `
		INSERT INTO holidays (telegram_chat_id, holiday_name, recipient_name, holiday_date, remind_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, telegram_chat_id, holiday_name, recipient_name, holiday_date, remind_at, status, created_at
	`

	var holiday database.HolidayView
	if err := d.pool.QueryRow(
		ctx,
		query,
		request.TelegramChatID,
		strings.TrimSpace(request.HolidayName),
		strings.TrimSpace(request.RecipientName),
		request.HolidayDate,
		request.RemindAt,
	).Scan(
		&holiday.ID,
		&holiday.TelegramChatID,
		&holiday.HolidayName,
		&holiday.RecipientName,
		&holiday.HolidayDate,
		&holiday.RemindAt,
		&holiday.Status,
		&holiday.CreatedAt,
	); err != nil {
		return database.HolidayView{}, fmt.Errorf("create holiday: %w", err)
	}

	return holiday, nil
}

func (d *Database) UpdateHoliday(
	ctx context.Context,
	request database.HolidayUpdateRequest,
) (database.HolidayView, error) {
	query := `
		UPDATE holidays
		SET holiday_name = $3,
			recipient_name = $4,
			holiday_date = $5,
			remind_at = $6,
			last_sent_at = NULL
		WHERE telegram_chat_id = $1
			AND id = $2
			AND status = 'active'
		RETURNING id, telegram_chat_id, holiday_name, recipient_name, holiday_date, remind_at, status, created_at
	`

	var holiday database.HolidayView
	if err := d.pool.QueryRow(
		ctx,
		query,
		request.TelegramChatID,
		request.ID,
		strings.TrimSpace(request.HolidayName),
		strings.TrimSpace(request.RecipientName),
		request.HolidayDate,
		request.RemindAt,
	).Scan(
		&holiday.ID,
		&holiday.TelegramChatID,
		&holiday.HolidayName,
		&holiday.RecipientName,
		&holiday.HolidayDate,
		&holiday.RemindAt,
		&holiday.Status,
		&holiday.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.HolidayView{}, fmt.Errorf("%w: %d", database.ErrHolidayNotFound, request.ID)
		}

		return database.HolidayView{}, fmt.Errorf("update holiday: %w", err)
	}

	return holiday, nil
}

func (d *Database) FindHolidaysByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]database.HolidayView, error) {
	query := `
		SELECT id, telegram_chat_id, holiday_name, recipient_name, holiday_date, remind_at, status, created_at
		FROM holidays
		WHERE telegram_chat_id = $1
			AND status = 'active'
		ORDER BY remind_at, id
		LIMIT $2 OFFSET $3
	`

	rows, err := d.pool.Query(ctx, query, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find holidays by chat: %w", err)
	}
	defer rows.Close()

	return scanHolidays(rows)
}

func (d *Database) HolidayByID(
	ctx context.Context,
	telegramChatID int64,
	holidayID int64,
) (database.HolidayView, error) {
	query := `
		SELECT id, telegram_chat_id, holiday_name, recipient_name, holiday_date, remind_at, status, created_at
		FROM holidays
		WHERE telegram_chat_id = $1
			AND id = $2
			AND status = 'active'
	`

	var holiday database.HolidayView
	if err := d.pool.QueryRow(ctx, query, telegramChatID, holidayID).Scan(
		&holiday.ID,
		&holiday.TelegramChatID,
		&holiday.HolidayName,
		&holiday.RecipientName,
		&holiday.HolidayDate,
		&holiday.RemindAt,
		&holiday.Status,
		&holiday.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.HolidayView{}, fmt.Errorf("%w: %d", database.ErrHolidayNotFound, holidayID)
		}

		return database.HolidayView{}, fmt.Errorf("load holiday by id: %w", err)
	}

	return holiday, nil
}

func (d *Database) DeleteHoliday(ctx context.Context, telegramChatID int64, holidayID int64) error {
	query := `
		UPDATE holidays
		SET status = 'canceled'
		WHERE telegram_chat_id = $1
			AND id = $2
			AND status = 'active'
	`

	if affectedRows, err := d.pool.Exec(ctx, query, telegramChatID, holidayID); err != nil {
		return fmt.Errorf("delete holiday: %w", err)
	} else if affectedRows.RowsAffected() == 0 {
		return fmt.Errorf("%w: %d", database.ErrHolidayNotFound, holidayID)
	}

	return nil
}

func (d *Database) FindHolidaysReadyToSend(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]*database.ScheduledHoliday, error) {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin find holidays ready to send transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "find holidays ready to send")

	query := `
		WITH due AS (
			SELECT id
			FROM holidays
			WHERE remind_at <= $1
				AND status = 'active'
			ORDER BY remind_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		SELECT
			h.id,
			h.telegram_chat_id,
			h.holiday_name,
			h.recipient_name,
			h.remind_at,
			COALESCE(s.timezone, 'Europe/Moscow')
		FROM holidays h
		JOIN due d ON d.id = h.id
		LEFT JOIN user_settings s ON s.telegram_chat_id = h.telegram_chat_id
		ORDER BY h.remind_at, h.id
	`

	rows, err := transaction.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("query holidays ready to send: %w", err)
	}
	defer rows.Close()

	holidays := make([]*database.ScheduledHoliday, 0)

	for rows.Next() {
		var holiday database.ScheduledHoliday
		if err = rows.Scan(
			&holiday.ID,
			&holiday.TelegramChatID,
			&holiday.HolidayName,
			&holiday.RecipientName,
			&holiday.RemindAt,
			&holiday.Timezone,
		); err != nil {
			return nil, fmt.Errorf("scan scheduled holiday: %w", err)
		}

		holidays = append(holidays, &holiday)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled holidays: %w", err)
	}

	if err = transaction.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit find holidays ready to send transaction: %w", err)
	}

	return holidays, nil
}

func scanHolidays(rows pgx.Rows) ([]database.HolidayView, error) {
	holidays := make([]database.HolidayView, 0)

	for rows.Next() {
		var holiday database.HolidayView
		if err := rows.Scan(
			&holiday.ID,
			&holiday.TelegramChatID,
			&holiday.HolidayName,
			&holiday.RecipientName,
			&holiday.HolidayDate,
			&holiday.RemindAt,
			&holiday.Status,
			&holiday.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan holiday: %w", err)
		}

		holidays = append(holidays, holiday)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate holidays: %w", err)
	}

	return holidays, nil
}
