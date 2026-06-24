package database

import (
	"errors"
	"time"
)

var ErrReminderNotFound = errors.New("reminder not found")
var ErrHolidayNotFound = errors.New("holiday not found")
var ErrShoppingListNotFound = errors.New("shopping list not found")

type Note struct {
	ID int64

	TelegramChatID int64

	Title       string
	Description string

	CreatedAt time.Time
}

type ReminderStatus string

const (
	ReminderStatusNew       ReminderStatus = "new"
	ReminderStatusSent      ReminderStatus = "sent"
	ReminderStatusFailed    ReminderStatus = "failed"
	ReminderStatusCanceled  ReminderStatus = "canceled"
	ReminderStatusCompleted ReminderStatus = "completed"
	ReminderStatusSkipped   ReminderStatus = "skipped"
)

type Reminder struct {
	ID     int64
	NoteID int64
	RuleID *int64

	RemindAt time.Time

	Status ReminderStatus

	Attempts          int
	SentAt            *time.Time
	LastSentAt        *time.Time
	NextSendAt        *time.Time
	IntrusiveUntil    *time.Time
	TelegramMessageID *int

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

	IntrusiveEnabled  bool
	IntrusiveInterval time.Duration
	Timezone          string
	TelegramMessageID *int
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

type ReminderStats struct {
	Completed int
	Pending   int
	Skipped   int
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
	WeekdaysMaskWorkdays = WeekdayMaskMonday |
		WeekdayMaskTuesday |
		WeekdayMaskWednesday |
		WeekdayMaskThursday |
		WeekdayMaskFriday
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

type UserSettings struct {
	TelegramChatID    int64
	Language          string
	Timezone          string
	IntrusiveEnabled  bool
	IntrusiveInterval time.Duration
	UpdatedAt         time.Time
}

type ReminderDelivery struct {
	ReminderID     int64
	MessageID      int
	NextSendAt     time.Time
	IntrusiveUntil time.Time
}

type HolidayStatus string

const (
	HolidayStatusActive   HolidayStatus = "active"
	HolidayStatusCanceled HolidayStatus = "canceled"
)

type Holiday struct {
	ID             int64
	TelegramChatID int64
	HolidayName    string
	RecipientName  string
	HolidayDate    time.Time
	RemindAt       time.Time
	Status         HolidayStatus
	LastSentAt     *time.Time
	CreatedAt      time.Time
}

type ScheduledHoliday struct {
	ID int64

	TelegramChatID int64
	HolidayName    string
	RecipientName  string
	RemindAt       time.Time
	Timezone       string
}

type HolidayRequest struct {
	TelegramChatID int64
	HolidayName    string
	RecipientName  string
	HolidayDate    time.Time
	RemindAt       time.Time
}

type HolidayUpdateRequest struct {
	HolidayRequest

	ID int64
}

type HolidayView struct {
	ID             int64
	TelegramChatID int64
	HolidayName    string
	RecipientName  string
	HolidayDate    time.Time
	RemindAt       time.Time
	Status         HolidayStatus
	CreatedAt      time.Time
}

type ShoppingListView struct {
	ID             int64
	TelegramChatID int64
	CreatedAt      time.Time
	Items          []ShoppingListItem
}

type ShoppingListItem struct {
	ID        int64
	ListID    int64
	Text      string
	Done      bool
	Position  int
	CreatedAt time.Time
}
