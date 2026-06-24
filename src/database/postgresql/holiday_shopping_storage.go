package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/EgorKo25/EasyNote/src/database"
)

func (d *Database) CreateHoliday(
	ctx context.Context,
	request database.HolidayRequest,
) (database.HolidayView, error) {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return database.HolidayView{}, fmt.Errorf("begin create holiday transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "create holiday")

	var noteID int64
	if err = transaction.QueryRow(
		ctx,
		"INSERT INTO notes (telegram_chat_id, title, description) VALUES ($1, $2, $3) RETURNING id",
		request.TelegramChatID,
		holidayReminderTitle(request.Name, request.PersonName),
		"",
	).Scan(&noteID); err != nil {
		return database.HolidayView{}, fmt.Errorf("create holiday note: %w", err)
	}

	var reminderID int64
	if err = transaction.QueryRow(
		ctx,
		"INSERT INTO reminders (note_id, remind_at) VALUES ($1, $2) RETURNING id",
		noteID,
		request.RemindAt,
	).Scan(&reminderID); err != nil {
		return database.HolidayView{}, fmt.Errorf("create holiday reminder: %w", err)
	}

	query := `
		INSERT INTO holidays (telegram_chat_id, reminder_id, name, person_name, holiday_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, telegram_chat_id, reminder_id, name, person_name, holiday_date, created_at
	`

	var holiday database.HolidayView
	if err = transaction.QueryRow(
		ctx,
		query,
		request.TelegramChatID,
		reminderID,
		strings.TrimSpace(request.Name),
		strings.TrimSpace(request.PersonName),
		request.HolidayDate,
	).Scan(
		&holiday.ID,
		&holiday.TelegramChatID,
		&holiday.ReminderID,
		&holiday.Name,
		&holiday.PersonName,
		&holiday.HolidayDate,
		&holiday.CreatedAt,
	); err != nil {
		return database.HolidayView{}, fmt.Errorf("create holiday: %w", err)
	}

	if err = transaction.Commit(ctx); err != nil {
		return database.HolidayView{}, fmt.Errorf("commit create holiday transaction: %w", err)
	}

	holiday.RemindAt = request.RemindAt

	return holiday, nil
}

func (d *Database) UpdateHoliday(
	ctx context.Context,
	request database.HolidayUpdateRequest,
) (database.HolidayView, error) {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return database.HolidayView{}, fmt.Errorf("begin update holiday transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "update holiday")

	holiday, err := d.holidayByIDTx(ctx, transaction, request.TelegramChatID, request.ID)
	if err != nil {
		return database.HolidayView{}, err
	}

	noteQuery := `
		UPDATE notes n
		SET title = $2,
			description = $3
		FROM reminders r
		WHERE r.note_id = n.id
			AND r.id = $1
	`

	if _, err = transaction.Exec(
		ctx,
		noteQuery,
		holiday.ReminderID,
		holidayReminderTitle(request.Name, request.PersonName),
		"",
	); err != nil {
		return database.HolidayView{}, fmt.Errorf("update holiday note: %w", err)
	}

	if _, err = transaction.Exec(
		ctx,
		"UPDATE reminders SET remind_at = $1 WHERE id = $2",
		request.RemindAt,
		holiday.ReminderID,
	); err != nil {
		return database.HolidayView{}, fmt.Errorf("update holiday reminder: %w", err)
	}

	query := `
		UPDATE holidays
		SET name = $3,
			person_name = $4,
			holiday_date = $5
		WHERE telegram_chat_id = $1
			AND id = $2
		RETURNING id, telegram_chat_id, reminder_id, name, person_name, holiday_date, created_at
	`

	if err = transaction.QueryRow(
		ctx,
		query,
		request.TelegramChatID,
		request.ID,
		strings.TrimSpace(request.Name),
		strings.TrimSpace(request.PersonName),
		request.HolidayDate,
	).Scan(
		&holiday.ID,
		&holiday.TelegramChatID,
		&holiday.ReminderID,
		&holiday.Name,
		&holiday.PersonName,
		&holiday.HolidayDate,
		&holiday.CreatedAt,
	); err != nil {
		return database.HolidayView{}, fmt.Errorf("update holiday: %w", err)
	}

	if err = transaction.Commit(ctx); err != nil {
		return database.HolidayView{}, fmt.Errorf("commit update holiday transaction: %w", err)
	}

	holiday.RemindAt = request.RemindAt

	return holiday, nil
}

func (d *Database) FindHolidaysByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]database.HolidayView, error) {
	query := `
		SELECT
			h.id,
			h.telegram_chat_id,
			h.reminder_id,
			h.name,
			h.person_name,
			h.holiday_date,
			r.remind_at,
			h.created_at
		FROM holidays h
		JOIN reminders r ON r.id = h.reminder_id
		WHERE h.telegram_chat_id = $1
		ORDER BY r.remind_at, h.id
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
	holiday, err := d.holidayByIDTx(ctx, d.pool, telegramChatID, holidayID)
	if err != nil {
		return database.HolidayView{}, err
	}

	return holiday, nil
}

func (d *Database) DeleteHoliday(ctx context.Context, telegramChatID int64, holidayID int64) error {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete holiday transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "delete holiday")

	holiday, err := d.holidayByIDTx(ctx, transaction, telegramChatID, holidayID)
	if err != nil {
		return err
	}

	if _, err = transaction.Exec(
		ctx,
		"UPDATE reminders SET status = 'canceled' WHERE id = $1",
		holiday.ReminderID,
	); err != nil {
		return fmt.Errorf("cancel holiday reminder: %w", err)
	}

	if _, err = transaction.Exec(ctx, "DELETE FROM holidays WHERE id = $1", holidayID); err != nil {
		return fmt.Errorf("delete holiday: %w", err)
	}

	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete holiday transaction: %w", err)
	}

	return nil
}

func (d *Database) CreateShoppingList(
	ctx context.Context,
	telegramChatID int64,
	items []database.ShoppingListItem,
) (database.ShoppingListView, error) {
	transaction, err := d.pool.Begin(ctx)
	if err != nil {
		return database.ShoppingListView{}, fmt.Errorf("begin create shopping list transaction: %w", err)
	}

	defer rollbackTransaction(ctx, transaction, "create shopping list")

	var list database.ShoppingListView
	if err = transaction.QueryRow(
		ctx,
		"INSERT INTO shopping_lists (telegram_chat_id) VALUES ($1) RETURNING id, telegram_chat_id, created_at",
		telegramChatID,
	).Scan(&list.ID, &list.TelegramChatID, &list.CreatedAt); err != nil {
		return database.ShoppingListView{}, fmt.Errorf("create shopping list: %w", err)
	}

	for index, item := range items {
		query := `
			INSERT INTO shopping_list_items (list_id, text, position)
			VALUES ($1, $2, $3)
			RETURNING id, list_id, text, done, position, created_at
		`

		var insertedItem database.ShoppingListItem
		if err = transaction.QueryRow(ctx, query, list.ID, strings.TrimSpace(item.Text), index+1).Scan(
			&insertedItem.ID,
			&insertedItem.ListID,
			&insertedItem.Text,
			&insertedItem.Done,
			&insertedItem.Position,
			&insertedItem.CreatedAt,
		); err != nil {
			return database.ShoppingListView{}, fmt.Errorf("create shopping list item: %w", err)
		}

		list.Items = append(list.Items, insertedItem)
	}

	if err = transaction.Commit(ctx); err != nil {
		return database.ShoppingListView{}, fmt.Errorf("commit create shopping list transaction: %w", err)
	}

	return list, nil
}

func (d *Database) FindShoppingListsByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]database.ShoppingListView, error) {
	query := `
		SELECT id, telegram_chat_id, created_at
		FROM shopping_lists
		WHERE telegram_chat_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := d.pool.Query(ctx, query, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find shopping lists by chat: %w", err)
	}
	defer rows.Close()

	lists, err := pgx.CollectRows(rows, pgx.RowToStructByName[database.ShoppingListView])
	if err != nil {
		return nil, fmt.Errorf("collect shopping lists by chat: %w", err)
	}

	return lists, nil
}

func (d *Database) ShoppingListByID(
	ctx context.Context,
	telegramChatID int64,
	listID int64,
) (database.ShoppingListView, error) {
	list, err := d.shoppingListByID(ctx, telegramChatID, listID)
	if err != nil {
		return database.ShoppingListView{}, err
	}

	items, err := d.shoppingListItems(ctx, list.ID)
	if err != nil {
		return database.ShoppingListView{}, err
	}

	list.Items = items

	return list, nil
}

func (d *Database) ToggleShoppingListItem(
	ctx context.Context,
	telegramChatID int64,
	itemID int64,
) (database.ShoppingListView, error) {
	var listID int64

	query := `
		UPDATE shopping_list_items i
		SET done = NOT i.done
		FROM shopping_lists l
		WHERE l.id = i.list_id
			AND l.telegram_chat_id = $1
			AND i.id = $2
		RETURNING i.list_id
	`

	if err := d.pool.QueryRow(ctx, query, telegramChatID, itemID).Scan(&listID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.ShoppingListView{}, fmt.Errorf("%w: item %d", database.ErrShoppingListNotFound, itemID)
		}

		return database.ShoppingListView{}, fmt.Errorf("toggle shopping list item: %w", err)
	}

	list, err := d.ShoppingListByID(ctx, telegramChatID, listID)
	if err != nil {
		return database.ShoppingListView{}, err
	}

	return list, nil
}

type holidayQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (d *Database) holidayByIDTx(
	ctx context.Context,
	queryer holidayQueryer,
	telegramChatID int64,
	holidayID int64,
) (database.HolidayView, error) {
	query := `
		SELECT
			h.id,
			h.telegram_chat_id,
			h.reminder_id,
			h.name,
			h.person_name,
			h.holiday_date,
			r.remind_at,
			h.created_at
		FROM holidays h
		JOIN reminders r ON r.id = h.reminder_id
		WHERE h.telegram_chat_id = $1
			AND h.id = $2
	`

	var holiday database.HolidayView
	if err := queryer.QueryRow(ctx, query, telegramChatID, holidayID).Scan(
		&holiday.ID,
		&holiday.TelegramChatID,
		&holiday.ReminderID,
		&holiday.Name,
		&holiday.PersonName,
		&holiday.HolidayDate,
		&holiday.RemindAt,
		&holiday.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.HolidayView{}, fmt.Errorf("%w: %d", database.ErrHolidayNotFound, holidayID)
		}

		return database.HolidayView{}, fmt.Errorf("load holiday by id: %w", err)
	}

	return holiday, nil
}

func scanHolidays(rows pgx.Rows) ([]database.HolidayView, error) {
	holidays := make([]database.HolidayView, 0)

	for rows.Next() {
		var holiday database.HolidayView
		if err := rows.Scan(
			&holiday.ID,
			&holiday.TelegramChatID,
			&holiday.ReminderID,
			&holiday.Name,
			&holiday.PersonName,
			&holiday.HolidayDate,
			&holiday.RemindAt,
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

func holidayReminderTitle(name string, personName string) string {
	return fmt.Sprintf("Поздравьте %s с %s.", strings.TrimSpace(personName), strings.TrimSpace(name))
}

func (d *Database) shoppingListByID(
	ctx context.Context,
	telegramChatID int64,
	listID int64,
) (database.ShoppingListView, error) {
	query := `
		SELECT id, telegram_chat_id, created_at
		FROM shopping_lists
		WHERE telegram_chat_id = $1
			AND id = $2
	`

	var list database.ShoppingListView
	if err := d.pool.QueryRow(ctx, query, telegramChatID, listID).Scan(
		&list.ID,
		&list.TelegramChatID,
		&list.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.ShoppingListView{}, fmt.Errorf("%w: %d", database.ErrShoppingListNotFound, listID)
		}

		return database.ShoppingListView{}, fmt.Errorf("load shopping list by id: %w", err)
	}

	return list, nil
}

func (d *Database) shoppingListItems(ctx context.Context, listID int64) ([]database.ShoppingListItem, error) {
	query := `
		SELECT id, list_id, text, done, position, created_at
		FROM shopping_list_items
		WHERE list_id = $1
		ORDER BY position, id
	`

	rows, err := d.pool.Query(ctx, query, listID)
	if err != nil {
		return nil, fmt.Errorf("load shopping list items: %w", err)
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[database.ShoppingListItem])
	if err != nil {
		return nil, fmt.Errorf("collect shopping list items: %w", err)
	}

	return items, nil
}
