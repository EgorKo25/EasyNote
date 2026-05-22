package database

import "time"

type Note struct {
	ID int64

	TelegramUserID int64
	TelegramChatID int64

	Text string

	CreatedAt time.Time
}

type ReminderStatus string

const (
	ReminderStatusPending    ReminderStatus = "pending"
	ReminderStatusProcessing ReminderStatus = "processing"
	ReminderStatusSent       ReminderStatus = "sent"
	ReminderStatusFailed     ReminderStatus = "failed"
	ReminderStatusCanceled   ReminderStatus = "canceled"
)

type Reminder struct {
	ID     int64
	NoteID int64

	RemindAt time.Time

	Status ReminderStatus

	Attempts    int
	LastError   *string
	LockedUntil *time.Time
	SentAt      *time.Time

	CreatedAt time.Time
}

type ScheduledReminder struct {
	ID int64

	NoteID int64

	TelegramChatID int64
	Text           string

	RemindAt time.Time

	Index int
}
