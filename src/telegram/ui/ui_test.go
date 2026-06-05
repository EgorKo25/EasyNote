package ui_test

import (
	"testing"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	telegramui "github.com/EgorKo25/EasyNote/src/telegram/ui"
	"github.com/mymmrac/telego"
	"github.com/stretchr/testify/require"
)

func TestTextRendering(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("MSK", 3*60*60)
	reminder := database.ReminderView{
		Title:       "Pay rent",
		Description: "Before lunch",
		RemindAt:    time.Date(2026, 6, 2, 6, 30, 0, 0, time.UTC),
	}

	tests := []struct {
		name     string
		action   func() string
		expected struct {
			text string
		}
	}{
		{
			name:   "main menu text explains keyboard action",
			action: telegramui.MainMenuText,
			expected: struct {
				text string
			}{text: "Главное меню\n\nВыберите действие на клавиатуре ниже."},
		},
		{
			name: "filled draft text shows reminder fields",
			action: func() string {
				return telegramui.DraftText(telegramui.Draft{
					Title:       "Pay rent",
					Description: "Before lunch",
					Date:        time.Date(2026, 6, 2, 0, 0, 0, 0, location),
					Hour:        9,
					Minute:      30,
				}, location)
			},
			expected: struct {
				text string
			}{
				text: "Создание напоминания\n\nДата и время: 02.06.2026 09:30\n" +
					"Название: Pay rent\nОписание: Before lunch\n" +
					"Повторяемость: не задана\n\nВыберите дату и время кнопкой ниже.",
			},
		},
		{
			name: "custom weekday repeat text shows selected count",
			action: func() string {
				return telegramui.RepeatText(telegramui.Draft{
					Repeat:       telegramui.RepeatCustomWeekdays,
					WeekdaysMask: 1 | 4 | 64,
				})
			},
			expected: struct {
				text string
			}{text: "Повторяемость: выбрано дней недели: 3\n"},
		},
		{
			name: "reminder view text includes formatted time",
			action: func() string {
				return telegramui.ReminderViewText(reminder, location)
			},
			expected: struct {
				text string
			}{text: "02.06.2026 09:30\nPay rent\nBefore lunch"},
		},
		{
			name: "empty reminder list text explains empty state",
			action: func() string {
				return telegramui.ReminderListText(nil)
			},
			expected: struct {
				text string
			}{text: "Активных напоминаний пока нет.\n\nСоздайте первое напоминание на основной клавиатуре."},
		},
		{
			name: "telegramui.Draft waiting for title changes next step",
			action: func() string {
				return telegramui.DraftText(telegramui.Draft{WaitingFor: telegramui.FieldTitle}, location)
			},
			expected: struct {
				text string
			}{
				text: "Создание напоминания\n\nДата и время: не задано\nНазвание: не задано\n" +
					"Описание: не задано\nПовторяемость: не задана\n\nВведите название напоминания сообщением.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expected.text, test.action())
		})
	}
}

func TestKeyboardRendering(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 8, 30, 0, 0, time.Local)

	tests := []struct {
		name     string
		action   func() *telego.InlineKeyboardMarkup
		expected struct {
			rows      int
			firstText string
		}
	}{
		{
			name: "empty one-time telegramui.Draft asks for date",
			action: func() *telego.InlineKeyboardMarkup {
				return telegramui.DraftKeyboard(telegramui.Draft{}, now)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 4, firstText: "Дата"},
		},
		{
			name: "filled recurring telegramui.Draft asks to change start date time",
			action: func() *telego.InlineKeyboardMarkup {
				return telegramui.DraftKeyboard(telegramui.Draft{
					Date:   time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local),
					Repeat: telegramui.RepeatEveryDay,
				}, now)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 4, firstText: "Изменить дату и время начала"},
		},
		{
			name: "date choice screen has quick choices",
			action: func() *telego.InlineKeyboardMarkup {
				return telegramui.DraftKeyboard(telegramui.Draft{Screen: telegramui.ScreenDate}, now)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 3, firstText: "Сегодня"},
		},
		{
			name: "calendar screen renders month grid",
			action: func() *telego.InlineKeyboardMarkup {
				return telegramui.DraftKeyboard(telegramui.Draft{
					Screen:        telegramui.ScreenCalendar,
					CalendarMonth: now,
				}, now)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 8, firstText: "<"},
		},
		{
			name: "time screen renders presets and steppers",
			action: func() *telego.InlineKeyboardMarkup {
				return telegramui.DraftKeyboard(telegramui.Draft{
					Screen: telegramui.ScreenTime,
					Date:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local),
					Hour:   9,
				}, now)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 4, firstText: "09:00"},
		},
		{
			name: "empty reminder list keeps main menu action",
			action: func() *telego.InlineKeyboardMarkup {
				return telegramui.ReminderListKeyboard(nil)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 1, firstText: "Главное меню"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			keyboard := test.action()

			require.Len(t, keyboard.InlineKeyboard, test.expected.rows)
			require.Equal(t, test.expected.firstText, keyboard.InlineKeyboard[0][0].Text)
		})
	}
}

func TestCalendarAndCallbacks(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("MSK", 3*60*60)

	tests := []struct {
		name     string
		action   func() string
		expected struct {
			text string
		}
	}{
		{
			name: "date callback encodes date",
			action: func() string {
				return telegramui.DateCallback(time.Date(2026, 6, 2, 0, 0, 0, 0, location))
			},
			expected: struct {
				text string
			}{text: "date:2026-06-02"},
		},
		{
			name: "month callback encodes month",
			action: func() string {
				return telegramui.MonthCallback(time.Date(2026, 6, 2, 0, 0, 0, 0, location))
			},
			expected: struct {
				text string
			}{text: "month:2026-06"},
		},
		{
			name: "time set callback encodes time",
			action: func() string {
				return telegramui.TimeSetCallback(9, 5)
			},
			expected: struct {
				text string
			}{text: "time:set:09:05"},
		},
		{
			name: "weekday callback encodes selected mask",
			action: func() string {
				return telegramui.WeekdayCallback(16)
			},
			expected: struct {
				text string
			}{text: "weekday:16"},
		},
		{
			name: "first day of month normalizes date",
			action: func() string {
				return telegramui.FirstDayOfMonth(time.Date(2026, 6, 23, 12, 0, 0, 0, location)).Format(time.DateOnly)
			},
			expected: struct {
				text string
			}{text: "2026-06-01"},
		},
		{
			name: "beginning of day normalizes time",
			action: func() string {
				return telegramui.BeginningOfDay(time.Date(2026, 6, 23, 12, 30, 0, 0, location)).Format(time.DateTime)
			},
			expected: struct {
				text string
			}{text: "2026-06-23 00:00:00"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expected.text, test.action())
		})
	}
}
