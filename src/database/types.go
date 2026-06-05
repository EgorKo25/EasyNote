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
	ReminderStatusNew      ReminderStatus = "new"
	ReminderStatusSent     ReminderStatus = "sent"
	ReminderStatusFailed   ReminderStatus = "failed"
	ReminderStatusCanceled ReminderStatus = "canceled"
)

type Reminder struct {
	ID     int64
	NoteID int64
	RuleID *int64

	RemindAt time.Time

	Status ReminderStatus

	Attempts int
	SentAt   *time.Time

	CreatedAt time.Time
}

type ScheduledReminder struct {
	ID int64

	NoteID int64
	RuleID *int64

	TelegramChatID int64
	Title          string
	Description    string

	RemindAt time.Time

	Index int
}

type ReminderView struct {
	ID     int64
	RuleID *int64

	Title       string
	Description string
	RemindAt    time.Time
	Status      ReminderStatus

	CreatedAt time.Time
}

type ReminderRuleType string

const (
	ReminderRuleTypeInterval    ReminderRuleType = "interval"
	ReminderRuleTypeWeekdays    ReminderRuleType = "weekdays"
	ReminderRuleTypeCustomDates ReminderRuleType = "custom_dates"
)

type ReminderRuleStatus string

const (
	ReminderRuleStatusActive    ReminderRuleStatus = "active"
	ReminderRuleStatusCanceled  ReminderRuleStatus = "canceled"
	ReminderRuleStatusCompleted ReminderRuleStatus = "completed"
)

const (
	WeekdayMaskMonday    = 1
	WeekdayMaskTuesday   = 2
	WeekdayMaskWednesday = 4
	WeekdayMaskThursday  = 8
	WeekdayMaskFriday    = 16
	WeekdayMaskSaturday  = 32
	WeekdayMaskSunday    = 64
	WeekdaysMaskWorkdays = WeekdayMaskMonday | WeekdayMaskTuesday | WeekdayMaskWednesday |
		WeekdayMaskThursday | WeekdayMaskFriday
)

type ReminderRule struct {
	ID           int64
	Type         ReminderRuleType
	IntervalDays *int
	WeekdaysMask *int
	Status       ReminderRuleStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CustomReminderDate struct {
	RuleID              int64
	RemindDate          time.Time
	GeneratedReminderID *int64
	CreatedAt           time.Time
}

type RecurringReminderRequest struct {
	TelegramChatID int64
	Title          string
	Description    string
	FirstRemindAt  time.Time
	Rule           ReminderRule
	CustomDates    []time.Time
}
