package model

import "time"

type ReminderStatus string

const (
	ReminderStatusNew       ReminderStatus = "new"
	ReminderStatusSent      ReminderStatus = "sent"
	ReminderStatusFailed    ReminderStatus = "failed"
	ReminderStatusCanceled  ReminderStatus = "canceled"
	ReminderStatusCompleted ReminderStatus = "completed"
	ReminderStatusSkipped   ReminderStatus = "skipped"
)

type ReminderView struct {
	ID          int64
	RuleID      *int64
	Title       string
	Description string
	RemindAt    time.Time
	Status      ReminderStatus
	CreatedAt   time.Time
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

type UserSettings struct {
	TelegramChatID    int64
	Language          string
	Timezone          string
	IntrusiveEnabled  bool
	IntrusiveInterval time.Duration
	UpdatedAt         time.Time
}

type HolidayStatus string

const (
	HolidayStatusActive   HolidayStatus = "active"
	HolidayStatusCanceled HolidayStatus = "canceled"
)

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
