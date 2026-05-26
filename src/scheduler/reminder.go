package scheduler

import "time"

type Reminder struct {
	TelegramUserID int64
	TelegramChatID int64
	Text           string
	RemindAt       time.Time
}
