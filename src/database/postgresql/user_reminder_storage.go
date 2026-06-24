package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/EgorKo25/EasyNote/src/database"
)

func (d *Database) UpsertBotUser(ctx context.Context, telegramChatID int64, username string) error {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return nil
	}

	query := `
		INSERT INTO bot_users (telegram_chat_id, username)
		VALUES ($1, $2)
		ON CONFLICT (telegram_chat_id) DO UPDATE
			SET username = EXCLUDED.username,
				updated_at = now()
	`

	if _, err := d.pool.Exec(ctx, query, telegramChatID, username); err != nil {
		return fmt.Errorf("upsert bot user: %w", err)
	}

	return nil
}

func (d *Database) BotUserByChatID(ctx context.Context, telegramChatID int64) (database.BotUser, error) {
	query := `
		SELECT telegram_chat_id, username, updated_at
		FROM bot_users
		WHERE telegram_chat_id = $1
	`

	var user database.BotUser
	if err := d.pool.QueryRow(ctx, query, telegramChatID).Scan(
		&user.TelegramChatID,
		&user.Username,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.BotUser{}, fmt.Errorf("%w: %d", database.ErrBotUserNotFound, telegramChatID)
		}

		return database.BotUser{}, fmt.Errorf("load bot user by chat id: %w", err)
	}

	return user, nil
}

func (d *Database) ReminderRecipientByUsername(
	ctx context.Context,
	ownerChatID int64,
	username string,
) (database.ReminderRecipient, error) {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")

	query := `
		SELECT
			u.telegram_chat_id,
			u.username,
			EXISTS (
				SELECT 1
				FROM reminder_permissions p
				WHERE p.owner_chat_id = u.telegram_chat_id
					AND p.sender_chat_id = $1
			) AS allowed,
			COALESCE(s.incoming_reminder_requests_enabled, true) AS incoming_enabled
		FROM bot_users u
		LEFT JOIN user_settings s ON s.telegram_chat_id = u.telegram_chat_id
		WHERE lower(u.username) = lower($2)
	`

	var recipient database.ReminderRecipient
	if err := d.pool.QueryRow(ctx, query, ownerChatID, username).Scan(
		&recipient.TelegramChatID,
		&recipient.Username,
		&recipient.Allowed,
		&recipient.IncomingEnabled,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.ReminderRecipient{}, fmt.Errorf("%w: @%s", database.ErrBotUserNotFound, username)
		}

		return database.ReminderRecipient{}, fmt.Errorf("load reminder recipient by username: %w", err)
	}

	return recipient, nil
}

func (d *Database) CreateSharedReminder(
	ctx context.Context,
	request database.SharedReminderRequest,
) (database.ReminderView, error) {
	query := `
		WITH created_note AS (
			INSERT INTO notes (
				telegram_chat_id,
				sender_chat_id,
				recipient_username,
				title,
				description
			)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		),
		created_reminder AS (
			INSERT INTO reminders (note_id, remind_at, status)
			SELECT id, $6, $7
			FROM created_note
			RETURNING id, remind_at, status, created_at
		)
		SELECT r.id, $4::text, $5::text, r.remind_at, r.status, $3::text, r.created_at
		FROM created_reminder r
	`

	var reminder database.ReminderView
	if err := d.pool.QueryRow(
		ctx,
		query,
		request.RecipientChatID,
		request.SenderChatID,
		request.RecipientUsername,
		request.Title,
		request.Description,
		request.RemindAt,
		request.Status,
	).Scan(
		&reminder.ID,
		&reminder.Title,
		&reminder.Description,
		&reminder.RemindAt,
		&reminder.Status,
		&reminder.RecipientUsername,
		&reminder.CreatedAt,
	); err != nil {
		return database.ReminderView{}, fmt.Errorf("create shared reminder: %w", err)
	}

	return reminder, nil
}

func (d *Database) AcceptIncomingReminder(ctx context.Context, ownerChatID int64, reminderID int64) error {
	query := `
		UPDATE reminders r
		SET status = 'new'
		FROM notes n
		WHERE n.id = r.note_id
			AND n.telegram_chat_id = $1
			AND r.id = $2
			AND r.status = 'pending_confirmation'
	`

	if affectedRows, err := d.pool.Exec(ctx, query, ownerChatID, reminderID); err != nil {
		return fmt.Errorf("accept incoming reminder: %w", err)
	} else if affectedRows.RowsAffected() == 0 {
		return fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
	}

	return nil
}

func (d *Database) DeclineIncomingReminder(ctx context.Context, ownerChatID int64, reminderID int64) error {
	query := `
		UPDATE reminders r
		SET status = 'declined'
		FROM notes n
		WHERE n.id = r.note_id
			AND n.telegram_chat_id = $1
			AND r.id = $2
			AND r.status = 'pending_confirmation'
	`

	if affectedRows, err := d.pool.Exec(ctx, query, ownerChatID, reminderID); err != nil {
		return fmt.Errorf("decline incoming reminder: %w", err)
	} else if affectedRows.RowsAffected() == 0 {
		return fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
	}

	return nil
}

func (d *Database) ReminderSenderChatID(ctx context.Context, ownerChatID int64, reminderID int64) (int64, error) {
	query := `
		SELECT n.sender_chat_id
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.telegram_chat_id = $1
			AND r.id = $2
	`

	var senderChatID int64
	if err := d.pool.QueryRow(ctx, query, ownerChatID, reminderID).Scan(&senderChatID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("%w: %d", database.ErrReminderNotFound, reminderID)
		}

		return 0, fmt.Errorf("load reminder sender chat id: %w", err)
	}

	return senderChatID, nil
}

func (d *Database) AllowReminderSender(ctx context.Context, ownerChatID int64, senderChatID int64) error {
	query := `
		INSERT INTO reminder_permissions (owner_chat_id, sender_chat_id)
		VALUES ($1, $2)
		ON CONFLICT (owner_chat_id, sender_chat_id) DO NOTHING
	`

	if _, err := d.pool.Exec(ctx, query, ownerChatID, senderChatID); err != nil {
		return fmt.Errorf("allow reminder sender: %w", err)
	}

	return nil
}

func (d *Database) RemoveReminderSender(ctx context.Context, ownerChatID int64, senderChatID int64) error {
	query := `
		DELETE FROM reminder_permissions
		WHERE owner_chat_id = $1
			AND sender_chat_id = $2
	`

	if _, err := d.pool.Exec(ctx, query, ownerChatID, senderChatID); err != nil {
		return fmt.Errorf("remove reminder sender: %w", err)
	}

	return nil
}

func (d *Database) FindReminderPermissions(
	ctx context.Context,
	ownerChatID int64,
	limit int,
	offset int,
) ([]database.ReminderPermissionView, error) {
	query := `
		SELECT u.telegram_chat_id, u.username, p.created_at
		FROM reminder_permissions p
		JOIN bot_users u ON u.telegram_chat_id = p.sender_chat_id
		WHERE p.owner_chat_id = $1
		ORDER BY p.created_at DESC, u.username
		LIMIT $2 OFFSET $3
	`

	rows, err := d.pool.Query(ctx, query, ownerChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find reminder permissions: %w", err)
	}

	permissions, err := pgx.CollectRows(rows, pgx.RowToStructByName[database.ReminderPermissionView])
	if err != nil {
		return nil, fmt.Errorf("collect reminder permissions: %w", err)
	}

	return permissions, nil
}

func (d *Database) FindOutgoingRemindersByChat(
	ctx context.Context,
	senderChatID int64,
	limit int,
	offset int,
) ([]database.ReminderView, error) {
	query := `
		SELECT r.id, r.rule_id, n.title, n.description, r.remind_at, r.status, n.recipient_username, r.created_at
		FROM reminders r
		JOIN notes n ON n.id = r.note_id
		WHERE n.sender_chat_id = $1
			AND n.telegram_chat_id <> $1
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := d.pool.Query(ctx, query, senderChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find outgoing reminders by chat: %w", err)
	}

	reminders, err := pgx.CollectRows(rows, pgx.RowToStructByName[database.ReminderView])
	if err != nil {
		return nil, fmt.Errorf("collect outgoing reminders by chat: %w", err)
	}

	return reminders, nil
}

func (d *Database) UpdateIncomingReminderRequests(ctx context.Context, telegramChatID int64, enabled bool) error {
	query := `
		INSERT INTO user_settings (telegram_chat_id, incoming_reminder_requests_enabled)
		VALUES ($1, $2)
		ON CONFLICT (telegram_chat_id) DO UPDATE
			SET incoming_reminder_requests_enabled = EXCLUDED.incoming_reminder_requests_enabled,
				updated_at = now()
	`

	if _, err := d.pool.Exec(ctx, query, telegramChatID, enabled); err != nil {
		return fmt.Errorf("update incoming reminder requests: %w", err)
	}

	return nil
}
