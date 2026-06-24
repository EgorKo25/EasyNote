package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/EgorKo25/EasyNote/src/database"
)

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
