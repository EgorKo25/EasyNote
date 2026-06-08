package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

const defaultIntrusiveInterval = 30 * time.Minute

func (d *Database) UserSettings(ctx context.Context, telegramChatID int64) (database.UserSettings, error) {
	query := `
		INSERT INTO user_settings (telegram_chat_id)
		VALUES ($1)
		ON CONFLICT (telegram_chat_id) DO UPDATE
			SET telegram_chat_id = EXCLUDED.telegram_chat_id
		RETURNING
			telegram_chat_id,
			language,
			timezone,
			intrusive_enabled,
			intrusive_interval_minutes,
			updated_at
	`

	var settings database.UserSettings

	var intervalMinutes int

	if err := d.pool.QueryRow(ctx, query, telegramChatID).Scan(
		&settings.TelegramChatID,
		&settings.Language,
		&settings.Timezone,
		&settings.IntrusiveEnabled,
		&intervalMinutes,
		&settings.UpdatedAt,
	); err != nil {
		return database.UserSettings{}, fmt.Errorf("load user settings: %w", err)
	}

	settings.IntrusiveInterval = time.Duration(intervalMinutes) * time.Minute

	return settings, nil
}

func (d *Database) UpdateUserTimezone(ctx context.Context, telegramChatID int64, timezone string) error {
	query := `
		INSERT INTO user_settings (telegram_chat_id, timezone)
		VALUES ($1, $2)
		ON CONFLICT (telegram_chat_id) DO UPDATE
			SET timezone = EXCLUDED.timezone,
				updated_at = now()
	`

	if _, err := d.pool.Exec(ctx, query, telegramChatID, timezone); err != nil {
		return fmt.Errorf("update user timezone: %w", err)
	}

	return nil
}

func (d *Database) UpdateIntrusiveSettings(
	ctx context.Context,
	telegramChatID int64,
	enabled bool,
	interval time.Duration,
) error {
	intervalMinutes := int(interval / time.Minute)
	if intervalMinutes == 0 {
		intervalMinutes = int(defaultIntrusiveInterval / time.Minute)
	}

	query := `
		INSERT INTO user_settings (telegram_chat_id, intrusive_enabled, intrusive_interval_minutes)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_chat_id) DO UPDATE
			SET intrusive_enabled = EXCLUDED.intrusive_enabled,
				intrusive_interval_minutes = EXCLUDED.intrusive_interval_minutes,
				updated_at = now()
	`

	if _, err := d.pool.Exec(ctx, query, telegramChatID, enabled, intervalMinutes); err != nil {
		return fmt.Errorf("update intrusive settings: %w", err)
	}

	return nil
}
