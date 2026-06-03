package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

var (
	ErrUnsupportedReminderRuleType = errors.New("unsupported reminder rule type")
	ErrInvalidIntervalRule         = errors.New("invalid interval reminder rule")
	ErrInvalidWeekdaysRule         = errors.New("invalid weekdays reminder rule")
	ErrNoMatchingWeekdays          = errors.New("weekdays reminder rule has no matching weekdays")
)

func CalculateNextReminder(input database.ReminderCompletionInput) (database.ReminderCompletionNext, error) {
	if input.Rule == nil || input.Rule.Status != database.ReminderRuleStatusActive {
		return database.ReminderCompletionNext{}, nil
	}

	switch input.Rule.Type {
	case database.ReminderRuleTypeInterval:
		return nextIntervalReminder(input.Reminder.RemindAt, input.Rule)
	case database.ReminderRuleTypeWeekdays:
		return nextWeekdayReminder(input.Reminder.RemindAt, input.Rule)
	case database.ReminderRuleTypeCustomDates:
		return nextCustomDateReminder(input.Reminder.RemindAt, input.CustomDates), nil
	default:
		return database.ReminderCompletionNext{}, fmt.Errorf("%w: %s", ErrUnsupportedReminderRuleType, input.Rule.Type)
	}
}

func nextIntervalReminder(remindAt time.Time, rule *database.ReminderRule) (database.ReminderCompletionNext, error) {
	if rule.IntervalDays == nil || *rule.IntervalDays <= 0 {
		return database.ReminderCompletionNext{}, ErrInvalidIntervalRule
	}

	next := remindAt.AddDate(0, 0, *rule.IntervalDays)

	return database.ReminderCompletionNext{RemindAt: &next}, nil
}

func nextWeekdayReminder(remindAt time.Time, rule *database.ReminderRule) (database.ReminderCompletionNext, error) {
	if rule.WeekdaysMask == nil || *rule.WeekdaysMask <= 0 {
		return database.ReminderCompletionNext{}, ErrInvalidWeekdaysRule
	}

	next := remindAt
	for range 7 {
		next = next.AddDate(0, 0, 1)
		if weekdayMask(next.Weekday())&*rule.WeekdaysMask != 0 {
			return database.ReminderCompletionNext{RemindAt: &next}, nil
		}
	}

	return database.ReminderCompletionNext{}, fmt.Errorf("%w: %d", ErrNoMatchingWeekdays, *rule.WeekdaysMask)
}

func nextCustomDateReminder(remindAt time.Time, dates []database.CustomReminderDate) database.ReminderCompletionNext {
	for _, date := range dates {
		if date.GeneratedReminderID != nil {
			continue
		}

		next := time.Date(
			date.RemindDate.Year(),
			date.RemindDate.Month(),
			date.RemindDate.Day(),
			remindAt.Hour(),
			remindAt.Minute(),
			remindAt.Second(),
			remindAt.Nanosecond(),
			remindAt.Location(),
		)

		return database.ReminderCompletionNext{RemindAt: &next}
	}

	return database.ReminderCompletionNext{CompleteRule: true}
}

func weekdayMask(weekday time.Weekday) int {
	switch weekday {
	case time.Monday:
		return 1
	case time.Tuesday:
		return 2
	case time.Wednesday:
		return 4
	case time.Thursday:
		return 8
	case time.Friday:
		return 16
	case time.Saturday:
		return 32
	case time.Sunday:
		return 64
	default:
		return 0
	}
}
