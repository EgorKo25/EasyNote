package database

import (
	"errors"
	"time"
)

var ErrReminderNotFound = errors.New("reminder not found")

type Note struct {
	ID int64

	TelegramChatID int64

	Title       string
	Description string

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

	Attempts int
	SentAt   *time.Time

	CreatedAt time.Time
}

type ScheduledReminder struct {
	ID int64

	NoteID int64

	TelegramChatID int64
	Title          string
	Description    string

	RemindAt time.Time

	Index int
}

type ReminderView struct {
	ID int64

	Title       string
	Description string
	RemindAt    time.Time
	Status      ReminderStatus

	CreatedAt time.Time
}
