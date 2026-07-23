package telegram

import "time"

type Reminder struct {
	ID int64

	NoteID int64
	RuleID *int64

	TelegramChatID int64
	Title          string
	Description    string

	RemindAt time.Time

	Index int

	IntrusiveEnabled  bool
	IntrusiveInterval time.Duration
	Timezone          string
	TelegramMessageID *int
}

type Holiday struct {
	ID int64

	TelegramChatID int64
	HolidayName    string
	RecipientName  string
	RemindAt       time.Time
	Timezone       string
}
