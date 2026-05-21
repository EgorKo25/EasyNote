package cache

import "time"

type ScheduledReminder struct {
	ID int64

	NoteID int64

	TelegramChatID int64
	Text           string

	RemindAt time.Time

	index int
}
