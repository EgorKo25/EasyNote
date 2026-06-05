package ui

import (
	"strconv"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func monthDayRows(month time.Time, now time.Time, selectedDates []time.Time) [][]telego.InlineKeyboardButton {
	firstDay := FirstDayOfMonth(month)
	lastDay := firstDay.AddDate(0, 1, -1).Day()
	weekdayOffset := int(firstDay.Weekday()+6) % 7
	rows := make([][]telego.InlineKeyboardButton, 0, 6)

	var row []telego.InlineKeyboardButton
	for range weekdayOffset {
		row = append(row, tu.InlineKeyboardButton(" ").WithCallbackData(CallbackNoop))
	}

	today := BeginningOfDay(now)

	for day := 1; day <= lastDay; day++ {
		date := time.Date(firstDay.Year(), firstDay.Month(), day, 0, 0, 0, 0, month.Location())

		callback := DateCallback(date)
		if date.Before(today) {
			callback = CallbackNoop
		}

		text := strconv.Itoa(day)
		if isDraftCustomDateSelected(date, selectedDates) {
			text = "✓ " + text
		}

		row = append(row, tu.InlineKeyboardButton(text).WithCallbackData(callback))

		if len(row) == 7 {
			rows = append(rows, row)
			row = nil
		}
	}

	if len(row) > 0 {
		for len(row) < 7 {
			row = append(row, tu.InlineKeyboardButton(" ").WithCallbackData(CallbackNoop))
		}

		rows = append(rows, row)
	}

	return rows
}

func FirstDayOfMonth(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
}

func isDraftCustomDateSelected(date time.Time, selectedDates []time.Time) bool {
	for _, selected := range selectedDates {
		if selected.Year() == date.Year() && selected.Month() == date.Month() && selected.Day() == date.Day() {
			return true
		}
	}

	return false
}

func isFutureDraftTime(draft Draft, hour int, minute int, now time.Time) bool {
	if draft.Date.IsZero() {
		return true
	}

	remindAt := time.Date(
		draft.Date.Year(),
		draft.Date.Month(),
		draft.Date.Day(),
		hour,
		minute,
		0,
		0,
		draft.Date.Location(),
	)

	return remindAt.After(now)
}

func BeginningOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
