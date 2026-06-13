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

func CreateKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(ButtonCreateReminderKind)),
		tu.KeyboardRow(tu.KeyboardButton(ButtonBack)),
	).WithResizeKeyboard()
}

func RemindersKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(ButtonReminderList), tu.KeyboardButton(ButtonReminderHistory)),
		tu.KeyboardRow(tu.KeyboardButton(ButtonBack)),
	).WithResizeKeyboard()
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
	titleButtonText := ButtonDraftTitle
	if draft.Title != "" {
		titleButtonText = ButtonDraftEditTitle
	}

	descriptionButtonText := ButtonDraftEditDescription

	dateButtonText := "Изменить дату"
	if draft.Repeat != RepeatNone {
		dateButtonText = "Изменить дату и время начала"
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(dateButtonText).WithCallbackData(CallbackDraftDate)),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(ButtonDraftRepeat).WithCallbackData(CallbackDraftRepeat),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(titleButtonText).WithCallbackData(CallbackDraftTitle),
			tu.InlineKeyboardButton(descriptionButtonText).WithCallbackData(CallbackDraftDescription),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(ButtonDraftSave).WithCallbackData(CallbackDraftSave),
			tu.InlineKeyboardButton(ButtonDraftCancel).WithCallbackData(CallbackDraftCancel),
		),
	)
}

func DraftReplyKeyboard(draft Draft, now time.Time) *telego.ReplyKeyboardMarkup {
	switch draft.Screen {
	case ScreenDate:
		return tu.Keyboard(
			tu.KeyboardRow(tu.KeyboardButton("Сегодня"), tu.KeyboardButton("Завтра")),
			tu.KeyboardRow(tu.KeyboardButton("Выбрать дату")),
			tu.KeyboardRow(tu.KeyboardButton(ButtonBack)),
		).WithResizeKeyboard()
	case ScreenTime:
		return tu.Keyboard(
			timePresetReplyRow(draft, now),
			tu.KeyboardRow(tu.KeyboardButton(ButtonMinusHour), tu.KeyboardButton(ButtonPlusHour)),
			tu.KeyboardRow(tu.KeyboardButton(ButtonMinusFiveMinutes), tu.KeyboardButton(ButtonPlusFiveMinutes)),
			tu.KeyboardRow(tu.KeyboardButton(ButtonDone), tu.KeyboardButton(ButtonBack)),
		).WithResizeKeyboard()
	case ScreenRepeat:
		return tu.Keyboard(
			tu.KeyboardRow(tu.KeyboardButton(ButtonRepeatEveryDay), tu.KeyboardButton(ButtonRepeatWeekdays)),
			tu.KeyboardRow(tu.KeyboardButton(ButtonRepeatCustomWeekdays), tu.KeyboardButton(ButtonRepeatInterval)),
			tu.KeyboardRow(tu.KeyboardButton(ButtonRepeatNone)),
		).WithResizeKeyboard()
	default:
		titleButtonText := ButtonDraftTitle
		if draft.Title != "" {
			titleButtonText = ButtonDraftEditTitle
		}

		descriptionButtonText := ButtonDraftEditDescription

		dateButtonText := "Изменить дату"

		return tu.Keyboard(
			tu.KeyboardRow(tu.KeyboardButton(dateButtonText), tu.KeyboardButton(ButtonDraftRepeat)),
			tu.KeyboardRow(tu.KeyboardButton(titleButtonText), tu.KeyboardButton(descriptionButtonText)),
			tu.KeyboardRow(tu.KeyboardButton(ButtonDraftSave)),
			tu.KeyboardRow(tu.KeyboardButton(ButtonDraftCancel)),
		).WithResizeKeyboard()
	}
}

func timePresetReplyRow(draft Draft, now time.Time) []telego.KeyboardButton {
	buttons := make([]telego.KeyboardButton, 0, 4)

	for _, preset := range []struct {
		hour   int
		minute int
	}{{hour: 9}, {hour: 12}, {hour: 18}, {hour: 21}} {
		if isFutureDraftTime(draft, preset.hour, preset.minute, now) {
			buttons = append(buttons, tu.KeyboardButton(fmt.Sprintf("%02d:%02d", preset.hour, preset.minute)))
		}
	}

	if len(buttons) == 0 {
		return tu.KeyboardRow(tu.KeyboardButton("Нет доступного времени"))
	}

	return buttons
}

func repeatKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Каждый день").WithCallbackData(RepeatCallback(RepeatEveryDay)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(ButtonRepeatWeekdays).WithCallbackData(RepeatCallback(RepeatWeekdays)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(ButtonRepeatCustomWeekdays).WithCallbackData(RepeatCallback(RepeatCustomWeekdays)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(ButtonRepeatInterval).WithCallbackData(RepeatCallback(RepeatCustomInterval)),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(ButtonRepeatNone).WithCallbackData(RepeatCallback(RepeatNone))),
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
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("День").WithCallbackData(CallbackDraftDate),
			tu.InlineKeyboardButton("Готово").WithCallbackData(CallbackDraftTimeDone),
		),
	)
}

func ReminderListKeyboard(reminders []database.ReminderView) *telego.ReplyKeyboardMarkup {
	rows := make([][]telego.KeyboardButton, 0, len(reminders)+2)
	for _, reminder := range reminders {
		rows = append(rows, tu.KeyboardRow(tu.KeyboardButton(ReminderButtonText(reminder))))
	}

	rows = append(rows, tu.KeyboardRow(tu.KeyboardButton(ButtonPreviousPage), tu.KeyboardButton(ButtonNextPage)))
	rows = append(rows, tu.KeyboardRow(tu.KeyboardButton(ButtonBack)))

	return tu.Keyboard(rows...).WithResizeKeyboard()
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

func SettingsKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(ButtonSettingsLanguage)),
		tu.KeyboardRow(tu.KeyboardButton(ButtonSettingsTimezone)),
		tu.KeyboardRow(tu.KeyboardButton(ButtonSettingsIntrusive)),
		tu.KeyboardRow(tu.KeyboardButton(ButtonBack)),
	).WithResizeKeyboard()
}

func TimezoneKeyboard(index int, now time.Time) *telego.ReplyKeyboardMarkup {
	index = normalizeTimezoneIndex(index)
	option := timezoneOptions[index]

	return tu.Keyboard(
		tu.KeyboardRow(
			tu.KeyboardButton(ButtonPreviousPage),
			tu.KeyboardButton(TimezoneButtonText(option, now)),
			tu.KeyboardButton(ButtonNextPage),
		),
		tu.KeyboardRow(tu.KeyboardButton(ButtonBack)),
	).WithResizeKeyboard()
}

func IntrusiveSettingsKeyboard(enabled bool) *telego.InlineKeyboardMarkup {
	toggleText := ButtonIntrusiveDisabled
	callback := CallbackSettingsIntrusiveOn

	if enabled {
		toggleText = ButtonIntrusiveEnabled
		callback = CallbackSettingsIntrusiveOff
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(toggleText).WithCallbackData(callback)),
	)
}

func ReminderButtonText(reminder database.ReminderView) string {
	return fmt.Sprintf("#%d %s", reminder.ID, reminder.Title)
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

func TimezoneIndexByShift(index int, shift int) int {
	return normalizeTimezoneIndex(index + shift)
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
