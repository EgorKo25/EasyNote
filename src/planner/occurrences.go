package planner

import (
	"errors"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

var (
	ErrInvalidIntervalRule = errors.New("invalid interval reminder rule")
	ErrInvalidWeekdaysRule = errors.New("invalid weekdays reminder rule")
	ErrNoMatchingWeekdays  = errors.New("weekdays reminder rule has no matching weekdays")
)

type NextOccurrence struct {
	RemindAt     *time.Time
	CompleteRule bool
}

func CalculateNextOccurrenceTime(
	currentRemindAt time.Time,
	rule database.ReminderRule,
	customDates []database.CustomReminderDate,
) (NextOccurrence, error) {
	if rule.Status != database.ReminderRuleStatusActive {
		return NextOccurrence{}, nil
	}

	switch rule.Type {
	case database.ReminderRuleTypeInterval:
		return nextIntervalOccurrence(currentRemindAt, rule)
	case database.ReminderRuleTypeWeekdays:
		return nextWeekdayOccurrence(currentRemindAt, rule)
	case database.ReminderRuleTypeCustomDates:
		return nextCustomDateOccurrence(currentRemindAt, customDates), nil
	default:
		return NextOccurrence{}, fmt.Errorf("%w: %s", ErrUnsupportedRuleType, rule.Type)
	}
}

func nextIntervalOccurrence(currentRemindAt time.Time, rule database.ReminderRule) (NextOccurrence, error) {
	if rule.IntervalDays == nil || *rule.IntervalDays <= 0 {
		return NextOccurrence{}, ErrInvalidIntervalRule
	}

	next := currentRemindAt.AddDate(0, 0, *rule.IntervalDays)

	return NextOccurrence{RemindAt: &next}, nil
}

func nextWeekdayOccurrence(currentRemindAt time.Time, rule database.ReminderRule) (NextOccurrence, error) {
	if rule.WeekdaysMask == nil || *rule.WeekdaysMask <= 0 {
		return NextOccurrence{}, ErrInvalidWeekdaysRule
	}

	next := currentRemindAt
	for range 7 {
		next = next.AddDate(0, 0, 1)
		if maskForWeekday(next.Weekday())&*rule.WeekdaysMask != 0 {
			return NextOccurrence{RemindAt: &next}, nil
		}
	}

	return NextOccurrence{}, fmt.Errorf("%w: %d", ErrNoMatchingWeekdays, *rule.WeekdaysMask)
}

func nextCustomDateOccurrence(currentRemindAt time.Time, dates []database.CustomReminderDate) NextOccurrence {
	for _, date := range dates {
		if date.GeneratedReminderID != nil {
			continue
		}

		next := time.Date(
			date.RemindDate.Year(),
			date.RemindDate.Month(),
			date.RemindDate.Day(),
			currentRemindAt.Hour(),
			currentRemindAt.Minute(),
			currentRemindAt.Second(),
			currentRemindAt.Nanosecond(),
			currentRemindAt.Location(),
		)

		return NextOccurrence{RemindAt: &next}
	}

	return NextOccurrence{CompleteRule: true}
}

func maskForWeekday(weekday time.Weekday) int {
	switch weekday {
	case time.Monday:
		return database.WeekdayMaskMonday
	case time.Tuesday:
		return database.WeekdayMaskTuesday
	case time.Wednesday:
		return database.WeekdayMaskWednesday
	case time.Thursday:
		return database.WeekdayMaskThursday
	case time.Friday:
		return database.WeekdayMaskFriday
	case time.Saturday:
		return database.WeekdayMaskSaturday
	case time.Sunday:
		return database.WeekdayMaskSunday
	default:
		return 0
	}
}
