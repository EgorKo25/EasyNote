package ui

import (
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func MainKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(ButtonCreateReminder)),
		tu.KeyboardRow(tu.KeyboardButton(ButtonReminders)),
		tu.KeyboardRow(tu.KeyboardButton(ButtonSettings)),
	).WithResizeKeyboard()
}

func RemindersKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(ButtonReminderList).WithCallbackData(CallbackReminderList),
			tu.InlineKeyboardButton(ButtonReminderHistory).WithCallbackData(CallbackReminderHistory),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Главное меню").WithCallbackData(CallbackMainMenu)),
	)
}

func DraftKeyboard(draft Draft, now time.Time) *telego.InlineKeyboardMarkup {
	switch draft.Screen {
	case ScreenDate:
		return dateChoiceKeyboard(now)
	case ScreenCalendar:
		return dateKeyboard(draft, now)
	case ScreenTime:
		return timeKeyboard(draft, now)
	case ScreenRepeat:
		return repeatKeyboard()
	case ScreenWeekdays:
		return weekdaysKeyboard(draft)
	case ScreenInterval:
		return intervalKeyboard(draft)
	default:
		return draftMainKeyboard(draft)
	}
}

func draftMainKeyboard(draft Draft) *telego.InlineKeyboardMarkup {
	titleButtonText := "Название"
	if draft.Title != "" {
		titleButtonText = "Изменить название"
	}

	descriptionButtonText := "Описание"
	if draft.Description != "" {
		descriptionButtonText = "Изменить описание"
	}

	dateButtonText := "Дата"
	if draft.Repeat != RepeatNone {
		dateButtonText = "Дата и время начала"
	}

	if draftDateIsSet(draft) {
		dateButtonText = "Изменить дату"
		if draft.Repeat != RepeatNone {
			dateButtonText = "Изменить дату и время начала"
		}
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(dateButtonText).WithCallbackData(CallbackDraftDate)),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Регулярное напоминание").WithCallbackData(CallbackDraftRepeat),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(titleButtonText).WithCallbackData(CallbackDraftTitle),
			tu.InlineKeyboardButton(descriptionButtonText).WithCallbackData(CallbackDraftDescription),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Сохранить").WithCallbackData(CallbackDraftSave),
			tu.InlineKeyboardButton("Отменить").WithCallbackData(CallbackDraftCancel),
		),
	)
}

func draftDateIsSet(draft Draft) bool {
	return !draft.Date.IsZero() || len(draft.CustomDates) > 0
}

func repeatKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Каждый день").WithCallbackData(RepeatCallback(RepeatEveryDay)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Свои дни недели").WithCallbackData(RepeatCallback(RepeatCustomWeekdays)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Интервал в днях").WithCallbackData(RepeatCallback(RepeatCustomInterval)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Без повторения").WithCallbackData(RepeatCallback(RepeatNone)),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(CallbackDraftBack)),
	)
}

func weekdaysKeyboard(draft Draft) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			weekdayButton("Пн", 1, draft.WeekdaysMask),
			weekdayButton("Вт", 2, draft.WeekdaysMask),
			weekdayButton("Ср", 4, draft.WeekdaysMask),
			weekdayButton("Чт", 8, draft.WeekdaysMask),
		),
		tu.InlineKeyboardRow(
			weekdayButton("Пт", 16, draft.WeekdaysMask),
			weekdayButton("Сб", 32, draft.WeekdaysMask),
			weekdayButton("Вс", 64, draft.WeekdaysMask),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Готово").WithCallbackData(CallbackDraftWeekdaysDone),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(CallbackDraftRepeat)),
	)
}

func intervalKeyboard(draft Draft) *telego.InlineKeyboardMarkup {
	valueText := "Интервал: не задан"
	if draft.IntervalDays > 0 {
		valueText = fmt.Sprintf("Интервал: %d дн.", draft.IntervalDays)
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(valueText).WithCallbackData(CallbackNoop)),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("-1").WithCallbackData(TimeShiftCallback(-1440)),
			tu.InlineKeyboardButton("+1").WithCallbackData(TimeShiftCallback(1440)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Готово").WithCallbackData(CallbackDraftIntervalDone),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(CallbackDraftRepeat)),
	)
}

func weekdayButton(text string, mask int, selectedMask int) telego.InlineKeyboardButton {
	if selectedMask&mask != 0 {
		text = "✓ " + text
	}

	return tu.InlineKeyboardButton(text).WithCallbackData(WeekdayCallback(mask))
}

func dateChoiceKeyboard(now time.Time) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Сегодня").WithCallbackData(DateCallback(now)),
			tu.InlineKeyboardButton("Завтра").WithCallbackData(DateCallback(now.AddDate(0, 0, 1))),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Выбрать дату").WithCallbackData(CallbackDraftCalendar)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(CallbackDraftBack)),
	)
}

func dateKeyboard(draft Draft, now time.Time) *telego.InlineKeyboardMarkup {
	month := draft.CalendarMonth
	if month.IsZero() {
		month = FirstDayOfMonth(now)
	}

	rows := [][]telego.InlineKeyboardButton{
		{
			tu.InlineKeyboardButton("<").WithCallbackData(MonthCallback(month.AddDate(0, -1, 0))),
			tu.InlineKeyboardButton(month.Format("01.2006")).WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton(">").WithCallbackData(MonthCallback(month.AddDate(0, 1, 0))),
		},
		{
			tu.InlineKeyboardButton("Пн").WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton("Вт").WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton("Ср").WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton("Чт").WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton("Пт").WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton("Сб").WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton("Вс").WithCallbackData(CallbackNoop),
		},
	}

	rows = append(rows, monthDayRows(month, now, draft.CustomDates)...)
	if draft.Repeat == RepeatCustomDates {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Готово").WithCallbackData(CallbackDraftCustomDone),
		))
	} else {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад к выбору даты").WithCallbackData(CallbackDraftDate),
		))
	}

	return tu.InlineKeyboard(rows...)
}

func timeKeyboard(draft Draft, now time.Time) *telego.InlineKeyboardMarkup {
	presetButtons := make([]telego.InlineKeyboardButton, 0, 4)

	for _, preset := range []struct {
		hour   int
		minute int
	}{
		{hour: 9},
		{hour: 12},
		{hour: 18},
		{hour: 21},
	} {
		if isFutureDraftTime(draft, preset.hour, preset.minute, now) {
			presetButtons = append(presetButtons,
				tu.InlineKeyboardButton(fmt.Sprintf("%02d:%02d", preset.hour, preset.minute)).
					WithCallbackData(TimeSetCallback(preset.hour, preset.minute)),
			)
		}
	}

	if len(presetButtons) == 0 {
		presetButtons = append(presetButtons,
			tu.InlineKeyboardButton("Нет доступного времени").WithCallbackData(CallbackNoop),
		)
	}

	return tu.InlineKeyboard(
		presetButtons,
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("-1ч").WithCallbackData(TimeShiftCallback(-60)),
			tu.InlineKeyboardButton(draft.SelectedTimeText()).WithCallbackData(CallbackNoop),
			tu.InlineKeyboardButton("+1ч").WithCallbackData(TimeShiftCallback(60)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("-5м").WithCallbackData(TimeShiftCallback(-5)),
			tu.InlineKeyboardButton("+5м").WithCallbackData(TimeShiftCallback(5)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("День").WithCallbackData(CallbackDraftDate),
			tu.InlineKeyboardButton("Готово").WithCallbackData(CallbackDraftTimeDone),
		),
	)
}

func ReminderListKeyboard(reminders []database.ReminderView) *telego.InlineKeyboardMarkup {
	rows := make([][]telego.InlineKeyboardButton, 0, len(reminders)+1)
	for _, reminder := range reminders {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(reminder.Title).
				WithCallbackData(fmt.Sprintf("%s%d", CallbackViewPrefix, reminder.ID)),
		))
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Главное меню").WithCallbackData(CallbackMainMenu),
	))

	return tu.InlineKeyboard(rows...)
}

func ReminderDetailsKeyboard(reminder database.ReminderView) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад").WithCallbackData(CallbackReminderBackList),
			tu.InlineKeyboardButton("Удалить").WithCallbackData(fmt.Sprintf("%s%d", CallbackDeletePrefix, reminder.ID)),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Главное меню").WithCallbackData(CallbackMainMenu)),
	)
}

func ReminderCreatedKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Главное меню").WithCallbackData(CallbackMainMenu),
			tu.InlineKeyboardButton("Список").WithCallbackData(CallbackReminderList),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Создать еще").WithCallbackData(CallbackCreateReminder)),
	)
}

func SettingsKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Язык: русский").WithCallbackData(CallbackSettingsLanguage)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Часовой пояс").WithCallbackData(CallbackSettingsTimezone)),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Настойчивые напоминания").WithCallbackData(CallbackSettingsIntrusive),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Главное меню").WithCallbackData(CallbackMainMenu)),
	)
}

func TimezoneKeyboard(index int, now time.Time) *telego.InlineKeyboardMarkup {
	index = normalizeTimezoneIndex(index)
	option := timezoneOptions[index]

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("<").WithCallbackData(TimezonePrevCallback(index)),
			tu.InlineKeyboardButton(TimezoneButtonText(option, now)).WithCallbackData(TimezoneSelectCallback(index)),
			tu.InlineKeyboardButton(">").WithCallbackData(TimezoneNextCallback(index)),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(CallbackSettings)),
	)
}

func IntrusiveSettingsKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Включить").WithCallbackData(CallbackSettingsIntrusiveOn),
			tu.InlineKeyboardButton("Выключить").WithCallbackData(CallbackSettingsIntrusiveOff),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(CallbackSettings)),
	)
}

func TimezoneIndex(timezone string) int {
	for index, option := range timezoneOptions {
		if option.Timezone == timezone {
			return index
		}
	}

	return 0
}

func TimezoneByIndex(index int) TimezoneOption {
	return timezoneOptions[normalizeTimezoneIndex(index)]
}

func TimezoneButtonText(option TimezoneOption, now time.Time) string {
	location, err := time.LoadLocation(option.Timezone)
	if err != nil {
		return option.City
	}

	_, offset := now.In(location).Zone()

	return fmt.Sprintf("%s (%+d)", option.City, offset/3600)
}

func TimezonePrevCallback(index int) string {
	return fmt.Sprintf("%s%d", CallbackSettingsTimezonePrev, normalizeTimezoneIndex(index-1))
}

func TimezoneNextCallback(index int) string {
	return fmt.Sprintf("%s%d", CallbackSettingsTimezoneNext, normalizeTimezoneIndex(index+1))
}

func TimezoneSelectCallback(index int) string {
	return fmt.Sprintf("%s%d", CallbackSettingsTimezoneSelect, normalizeTimezoneIndex(index))
}

func normalizeTimezoneIndex(index int) int {
	count := len(timezoneOptions)
	if count == 0 {
		return 0
	}

	index %= count
	if index < 0 {
		index += count
	}

	return index
}

var timezoneOptions = []TimezoneOption{
	{City: "Москва", Timezone: "Europe/Moscow"},
	{City: "Калининград", Timezone: "Europe/Kaliningrad"},
	{City: "Самара", Timezone: "Europe/Samara"},
	{City: "Екатеринбург", Timezone: "Asia/Yekaterinburg"},
	{City: "Омск", Timezone: "Asia/Omsk"},
	{City: "Новосибирск", Timezone: "Asia/Novosibirsk"},
	{City: "Красноярск", Timezone: "Asia/Krasnoyarsk"},
	{City: "Иркутск", Timezone: "Asia/Irkutsk"},
	{City: "Якутск", Timezone: "Asia/Yakutsk"},
	{City: "Владивосток", Timezone: "Asia/Vladivostok"},
	{City: "Магадан", Timezone: "Asia/Magadan"},
	{City: "Камчатка", Timezone: "Asia/Kamchatka"},
}
