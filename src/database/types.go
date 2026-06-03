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

const WeekdaysMaskWorkdays = 31

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

type ReminderCompletionStatus string

const (
	ReminderCompletionStatusSent   ReminderCompletionStatus = "sent"
	ReminderCompletionStatusFailed ReminderCompletionStatus = "failed"
)

type ReminderCompletionRequest struct {
	ReminderID  int64
	Status      ReminderCompletionStatus
	SentAt      *time.Time
	MaxAttempts int
	Next        ReminderCompletionCalculator
}

type ReminderCompletionInput struct {
	Reminder    ScheduledReminder
	Rule        *ReminderRule
	CustomDates []CustomReminderDate
}

type ReminderCompletionNext struct {
	RemindAt     *time.Time
	CompleteRule bool
}

type ReminderCompletionCalculator func(ReminderCompletionInput) (ReminderCompletionNext, error)
