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
			name:   "main menu text stays visually empty",
			action: telegramui.MainMenuText,
			expected: struct {
				text string
			}{text: " "},
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
					"Повторять: нет\n\nВыберите дату и время кнопкой ниже.",
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
			}{text: "Повторять: выбрано дней недели: 3\n"},
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
					"Описание: не задано\nПовторять: нет\n\nВведите название напоминания сообщением.",
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
		action   func() any
		expected struct {
			rows      int
			firstText string
		}
	}{
		{
			name: "empty one-time telegramui.Draft asks for date",
			action: func() any {
				return telegramui.DraftKeyboard(telegramui.Draft{}, now)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 4, firstText: "Дата"},
		},
		{
			name: "filled recurring telegramui.Draft asks to change start date time",
			action: func() any {
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
			action: func() any {
				return telegramui.DraftKeyboard(telegramui.Draft{Screen: telegramui.ScreenDate}, now)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 3, firstText: "Сегодня"},
		},
		{
			name: "calendar screen renders month grid",
			action: func() any {
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
			action: func() any {
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
			name: "empty reminder list keeps pagination and back actions",
			action: func() any {
				return telegramui.ReminderListKeyboard(nil)
			},
			expected: struct {
				rows      int
				firstText string
			}{rows: 2, firstText: "<-"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rows := keyboardTexts(test.action())

			require.Len(t, rows, test.expected.rows)
			require.Equal(t, test.expected.firstText, rows[0][0])
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

func TestMenuAndActionKeyboards(t *testing.T) {
	t.Parallel()

	reminder := database.ReminderView{ID: 7, Title: "Pay rent"}

	tests := []struct {
		name     string
		action   func() [][]string
		expected struct {
			rows [][]string
		}
	}{
		{
			name: "main keyboard exposes create reminders and settings",
			action: func() [][]string {
				keyboard := telegramui.MainKeyboard()

				return keyboardTexts(keyboard.Keyboard)
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Создать"}, {"Напоминания"}, {"Настройки"}}},
		},
		{
			name: "reminders keyboard exposes active and past lists",
			action: func() [][]string {
				return keyboardTexts(telegramui.RemindersKeyboard().Keyboard)
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Активные", "Прошедшие"}, {"Назад"}}},
		},
		{
			name: "reminder details keyboard exposes back delete and main menu",
			action: func() [][]string {
				return keyboardTexts(telegramui.ReminderDetailsKeyboard(reminder).InlineKeyboard)
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Назад", "Удалить"}, {"Главное меню"}}},
		},
		{
			name: "created reminder keyboard exposes navigation actions",
			action: func() [][]string {
				return keyboardTexts(telegramui.ReminderCreatedKeyboard().InlineKeyboard)
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Главное меню", "Список"}, {"Создать еще"}}},
		},
		{
			name: "settings keyboard exposes user setting actions",
			action: func() [][]string {
				return keyboardTexts(telegramui.SettingsKeyboard().Keyboard)
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Язык: русский"}, {"Часовой пояс"}, {"Настойчивые напоминания"}, {"Назад"}}},
		},
		{
			name: "intrusive settings keyboard exposes enable disable and back",
			action: func() [][]string {
				return keyboardTexts(telegramui.IntrusiveSettingsKeyboard(false).Keyboard)
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Выключено"}, {"Интервал"}, {"Назад"}}},
		},
		{
			name: "repeat draft screen exposes repeat choices",
			action: func() [][]string {
				keyboard := telegramui.DraftKeyboard(telegramui.Draft{Screen: telegramui.ScreenRepeat}, time.Now())

				return keyboardTexts(keyboard.InlineKeyboard[:4])
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Каждый день"}, {"По будням"}, {"Свои дни недели"}, {"Интервал в днях"}}},
		},
		{
			name: "weekdays draft screen marks selected weekdays",
			action: func() [][]string {
				keyboard := telegramui.DraftKeyboard(telegramui.Draft{
					Screen:       telegramui.ScreenWeekdays,
					WeekdaysMask: 1 | 16,
				}, time.Now())

				return keyboardTexts(keyboard.InlineKeyboard[:2])
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"✓ Пн", "Вт", "Ср", "Чт"}, {"✓ Пт", "Сб", "Вс"}}},
		},
		{
			name: "interval draft screen shows selected interval",
			action: func() [][]string {
				keyboard := telegramui.DraftKeyboard(telegramui.Draft{
					Screen:       telegramui.ScreenInterval,
					IntervalDays: 3,
				}, time.Now())

				return keyboardTexts(keyboard.InlineKeyboard[:1])
			},
			expected: struct {
				rows [][]string
			}{rows: [][]string{{"Интервал: 3 дн."}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expected.rows, test.action())
		})
	}
}

func TestSettingsHistoryAndStatusTexts(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("MSK", 3*60*60)
	interval := 45 * time.Minute

	tests := []struct {
		name     string
		action   func() string
		expected struct {
			text string
		}
	}{
		{
			name:   "reminders text asks to choose list",
			action: telegramui.RemindersText,
			expected: struct {
				text string
			}{text: "Напоминания\n\nВыберите список напоминаний."},
		},
		{
			name: "history text renders completed reminder",
			action: func() string {
				return telegramui.ReminderHistoryText([]database.ReminderView{{
					Title:    "Pay rent",
					RemindAt: time.Date(2026, 6, 2, 6, 30, 0, 0, time.UTC),
					Status:   database.ReminderStatusCompleted,
				}})
			},
			expected: struct {
				text string
			}{text: "Прошедшие напоминания\n\nВыберите напоминание из списка."},
		},
		{
			name: "settings text renders enabled intrusive settings",
			action: func() string {
				return telegramui.SettingsText(database.UserSettings{
					Timezone:          "Europe/Moscow",
					IntrusiveEnabled:  true,
					IntrusiveInterval: interval,
				})
			},
			expected: struct {
				text string
			}{
				text: "Настройки\n\nЯзык: русский\nЧасовой пояс: Europe/Moscow\n" +
					"Настойчивые напоминания: включены\nИнтервал: 45m0s",
			},
		},
		{
			name: "intrusive settings text renders disabled state",
			action: func() string {
				return telegramui.IntrusiveSettingsText(database.UserSettings{
					IntrusiveEnabled:  false,
					IntrusiveInterval: 30 * time.Minute,
				})
			},
			expected: struct {
				text string
			}{text: "Настойчивые напоминания\n\nСостояние: выключены\nИнтервал: 30m0s"},
		},
		{
			name: "failed status text is localized",
			action: func() string {
				return telegramui.ReminderStatusText(database.ReminderStatusFailed)
			},
			expected: struct {
				text string
			}{text: "Ошибка отправки"},
		},
		{
			name: "unknown status text returns raw value",
			action: func() string {
				return telegramui.ReminderStatusText(database.ReminderStatus("paused"))
			},
			expected: struct {
				text string
			}{text: "paused"},
		},
		{
			name: "custom interval repeat text renders interval days",
			action: func() string {
				return telegramui.RepeatText(telegramui.Draft{
					Repeat:       telegramui.RepeatCustomInterval,
					IntervalDays: 5,
				})
			},
			expected: struct {
				text string
			}{text: "Повторять: каждые 5 дн.\n"},
		},
		{
			name: "custom dates repeat text renders selected count",
			action: func() string {
				return telegramui.RepeatText(telegramui.Draft{
					Repeat:      telegramui.RepeatCustomDates,
					CustomDates: []time.Time{time.Date(2026, 6, 2, 0, 0, 0, 0, location)},
				})
			},
			expected: struct {
				text string
			}{text: "Повторять: выбрано дат: 1\n"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expected.text, test.action())
		})
	}
}

func TestTimezoneControls(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		action   func() string
		expected struct {
			text string
		}
	}{
		{
			name: "moscow button text shows positive utc offset",
			action: func() string {
				return telegramui.TimezoneButtonText(telegramui.TimezoneByIndex(0), now)
			},
			expected: struct {
				text string
			}{text: "Москва (+3)"},
		},
		{
			name: "next callback advances timezone index",
			action: func() string {
				return telegramui.TimezoneNextCallback(0)
			},
			expected: struct {
				text string
			}{text: "settings:timezone:next:1"},
		},
		{
			name: "previous callback decrements timezone index",
			action: func() string {
				return telegramui.TimezonePrevCallback(1)
			},
			expected: struct {
				text string
			}{text: "settings:timezone:prev:0"},
		},
		{
			name: "select callback stores timezone index",
			action: func() string {
				return telegramui.TimezoneSelectCallback(2)
			},
			expected: struct {
				text string
			}{text: "settings:timezone:select:2"},
		},
		{
			name: "negative timezone index wraps to last option",
			action: func() string {
				return telegramui.TimezonePrevCallback(0)
			},
			expected: struct {
				text string
			}{text: "settings:timezone:prev:11"},
		},
		{
			name: "large timezone index wraps through options",
			action: func() string {
				return telegramui.TimezoneButtonText(telegramui.TimezoneByIndex(99), now)
			},
			expected: struct {
				text string
			}{text: "Екатеринбург (+5)"},
		},
		{
			name: "timezone lookup finds configured city index",
			action: func() string {
				return telegramui.TimezoneSelectCallback(telegramui.TimezoneIndex("Asia/Novosibirsk"))
			},
			expected: struct {
				text string
			}{text: "settings:timezone:select:5"},
		},
		{
			name: "timezone keyboard exposes arrows and selected city",
			action: func() string {
				rows := keyboardTexts(telegramui.TimezoneKeyboard(0, now).Keyboard)

				return rows[0][0] + rows[0][1] + rows[0][2]
			},
			expected: struct {
				text string
			}{text: "<-Москва (+3)->"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expected.text, test.action())
		})
	}
}

func keyboardTexts(rows any) [][]string {
	switch typedRows := rows.(type) {
	case *telego.InlineKeyboardMarkup:
		return keyboardTexts(typedRows.InlineKeyboard)
	case *telego.ReplyKeyboardMarkup:
		return keyboardTexts(typedRows.Keyboard)
	case [][]telego.KeyboardButton:
		texts := make([][]string, 0, len(typedRows))
		for _, row := range typedRows {
			textRow := make([]string, 0, len(row))
			for _, button := range row {
				textRow = append(textRow, button.Text)
			}

			texts = append(texts, textRow)
		}

		return texts
	case [][]telego.InlineKeyboardButton:
		texts := make([][]string, 0, len(typedRows))
		for _, row := range typedRows {
			textRow := make([]string, 0, len(row))
			for _, button := range row {
				textRow = append(textRow, button.Text)
			}

			texts = append(texts, textRow)
		}

		return texts
	default:
		return nil
	}
}
