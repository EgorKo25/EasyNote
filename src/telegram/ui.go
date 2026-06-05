package telegram

import (
	"fmt"
	"strconv"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	buttonStart          = "Старт"
	buttonCreateReminder = "Создать напоминание"
	buttonReminderList   = "Список напоминаний"
)

const (
	callbackDraftTitle        = "draft:title"
	callbackDraftDescription  = "draft:description"
	callbackDraftDate         = "draft:date"
	callbackDraftCalendar     = "draft:calendar"
	callbackDraftRepeat       = "draft:repeat"
	callbackDraftBack         = "draft:back"
	callbackDraftSave         = "draft:save"
	callbackDraftCancel       = "draft:cancel"
	callbackDraftTimeDone     = "draft:time_done"
	callbackDraftCustomDone   = "draft:custom_dates_done"
	callbackDraftWeekdaysDone = "draft:weekdays_done"
	callbackNoop              = "noop"
	callbackMainMenu          = "menu:main"
	callbackCreateReminder    = "reminder:create"
	callbackReminderList      = "reminder:list"
	callbackReminderBackList  = "reminder:back_list"
	callbackDatePrefix        = "date:"
	callbackMonthPrefix       = "month:"
	callbackTimeSetPrefix     = "time:set:"
	callbackTimeShiftPrefix   = "time:shift:"
	callbackRepeatPrefix      = "repeat:"
	callbackWeekdayPrefix     = "weekday:"
	callbackViewPrefix        = "reminder:view:"
	callbackDeletePrefix      = "reminder:delete:"
)

const (
	fieldTitle       draftField = "title"
	fieldDescription draftField = "description"
)

const (
	draftScreenMain     draftScreen = "main"
	draftScreenDate     draftScreen = "date"
	draftScreenCalendar draftScreen = "calendar"
	draftScreenTime     draftScreen = "time"
	draftScreenRepeat   draftScreen = "repeat"
	draftScreenWeekdays draftScreen = "weekdays"
)

const emptyFieldText = "не задано"

type reminderRepeat string

const (
	reminderRepeatNone           reminderRepeat = ""
	reminderRepeatEveryDay       reminderRepeat = "every_day"
	reminderRepeatEveryWeek      reminderRepeat = "every_week"
	reminderRepeatEveryTwoDays   reminderRepeat = "every_two_days"
	reminderRepeatWeekdays       reminderRepeat = "weekdays"
	reminderRepeatCustomWeekdays reminderRepeat = "custom_weekdays"
	reminderRepeatCustomDates    reminderRepeat = "custom_dates"
)

func mainKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(buttonCreateReminder)),
		tu.KeyboardRow(tu.KeyboardButton(buttonReminderList)),
	).WithResizeKeyboard()
}

func draftKeyboard(draft reminderDraft, now time.Time) *telego.InlineKeyboardMarkup {
	switch draft.screen {
	case draftScreenDate:
		return dateChoiceKeyboard(now)
	case draftScreenCalendar:
		return dateKeyboard(draft, now)
	case draftScreenTime:
		return timeKeyboard(draft, now)
	case draftScreenRepeat:
		return repeatKeyboard()
	case draftScreenWeekdays:
		return weekdaysKeyboard(draft)
	default:
		return draftMainKeyboard(draft)
	}
}

func draftMainKeyboard(draft reminderDraft) *telego.InlineKeyboardMarkup {
	timeButtonText := "Задать дату и время"
	if !draft.date.IsZero() {
		timeButtonText = "Изменить дату и время"
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(timeButtonText).WithCallbackData(callbackDraftDate)),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Регулярное напоминание").WithCallbackData(callbackDraftRepeat),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Название").WithCallbackData(callbackDraftTitle),
			tu.InlineKeyboardButton("Описание").WithCallbackData(callbackDraftDescription),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Сохранить").WithCallbackData(callbackDraftSave),
			tu.InlineKeyboardButton("Отменить").WithCallbackData(callbackDraftCancel),
		),
	)
}

func repeatKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Каждый день").WithCallbackData(repeatCallback(reminderRepeatEveryDay)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Каждую неделю").WithCallbackData(repeatCallback(reminderRepeatEveryWeek)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Каждые 2 дня").WithCallbackData(repeatCallback(reminderRepeatEveryTwoDays)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("По будням").WithCallbackData(repeatCallback(reminderRepeatWeekdays)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Свои дни недели").WithCallbackData(
				repeatCallback(reminderRepeatCustomWeekdays),
			),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Выбрать даты").WithCallbackData(repeatCallback(reminderRepeatCustomDates)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Без повторения").WithCallbackData(repeatCallback(reminderRepeatNone)),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(callbackDraftBack)),
	)
}

func weekdaysKeyboard(draft reminderDraft) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			weekdayButton("Пн", 1, draft.weekdaysMask),
			weekdayButton("Вт", 2, draft.weekdaysMask),
			weekdayButton("Ср", 4, draft.weekdaysMask),
			weekdayButton("Чт", 8, draft.weekdaysMask),
		),
		tu.InlineKeyboardRow(
			weekdayButton("Пт", 16, draft.weekdaysMask),
			weekdayButton("Сб", 32, draft.weekdaysMask),
			weekdayButton("Вс", 64, draft.weekdaysMask),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Готово").WithCallbackData(callbackDraftWeekdaysDone),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(callbackDraftRepeat)),
	)
}

func weekdayButton(text string, mask int, selectedMask int) telego.InlineKeyboardButton {
	if selectedMask&mask != 0 {
		text = "✓ " + text
	}

	return tu.InlineKeyboardButton(text).WithCallbackData(weekdayCallback(mask))
}

func dateChoiceKeyboard(now time.Time) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Сегодня").WithCallbackData(dateCallback(now)),
			tu.InlineKeyboardButton("Завтра").WithCallbackData(dateCallback(now.AddDate(0, 0, 1))),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Выбрать дату").WithCallbackData(callbackDraftCalendar)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(callbackDraftBack)),
	)
}

func dateKeyboard(draft reminderDraft, now time.Time) *telego.InlineKeyboardMarkup {
	month := draft.calendarMonth
	if month.IsZero() {
		month = firstDayOfMonth(now)
	}

	rows := [][]telego.InlineKeyboardButton{
		{
			tu.InlineKeyboardButton("<").WithCallbackData(monthCallback(month.AddDate(0, -1, 0))),
			tu.InlineKeyboardButton(month.Format("01.2006")).WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton(">").WithCallbackData(monthCallback(month.AddDate(0, 1, 0))),
		},
		{
			tu.InlineKeyboardButton("Пн").WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("Вт").WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("Ср").WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("Чт").WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("Пт").WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("Сб").WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("Вс").WithCallbackData(callbackNoop),
		},
	}

	rows = append(rows, monthDayRows(month, now, draft.customDates)...)
	if draft.repeat == reminderRepeatCustomDates {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Готово").WithCallbackData(callbackDraftCustomDone),
		))
	} else {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад к выбору даты").WithCallbackData(callbackDraftDate),
		))
	}

	return tu.InlineKeyboard(rows...)
}

func timeKeyboard(draft reminderDraft, now time.Time) *telego.InlineKeyboardMarkup {
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
					WithCallbackData(timeSetCallback(preset.hour, preset.minute)),
			)
		}
	}

	if len(presetButtons) == 0 {
		presetButtons = append(presetButtons,
			tu.InlineKeyboardButton("Нет доступного времени").WithCallbackData(callbackNoop),
		)
	}

	return tu.InlineKeyboard(
		presetButtons,
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("-1ч").WithCallbackData(timeShiftCallback(-60)),
			tu.InlineKeyboardButton(draft.selectedTimeText()).WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("+1ч").WithCallbackData(timeShiftCallback(60)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("-5м").WithCallbackData(timeShiftCallback(-5)),
			tu.InlineKeyboardButton("+5м").WithCallbackData(timeShiftCallback(5)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("День").WithCallbackData(callbackDraftDate),
			tu.InlineKeyboardButton("Готово").WithCallbackData(callbackDraftTimeDone),
		),
	)
}

func reminderListKeyboard(reminders []database.ReminderView) *telego.InlineKeyboardMarkup {
	rows := make([][]telego.InlineKeyboardButton, 0, len(reminders)+1)
	for _, reminder := range reminders {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(reminder.Title).
				WithCallbackData(fmt.Sprintf("%s%d", callbackViewPrefix, reminder.ID)),
		))
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Главное меню").WithCallbackData(callbackMainMenu),
	))

	return tu.InlineKeyboard(rows...)
}

func reminderDetailsKeyboard(reminder database.ReminderView) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад").WithCallbackData(callbackReminderBackList),
			tu.InlineKeyboardButton("Удалить").WithCallbackData(fmt.Sprintf("%s%d", callbackDeletePrefix, reminder.ID)),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Главное меню").WithCallbackData(callbackMainMenu)),
	)
}

func reminderCreatedKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Главное меню").WithCallbackData(callbackMainMenu),
			tu.InlineKeyboardButton("Список").WithCallbackData(callbackReminderList),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Создать еще").WithCallbackData(callbackCreateReminder)),
	)
}

func mainMenuText() string {
	return "Главное меню\n\nВыберите действие на клавиатуре ниже."
}

func draftText(draft reminderDraft, location *time.Location) string {
	remindAt, title, description := emptyFieldText, emptyFieldText, emptyFieldText
	repeat := repeatText(draft)

	if !draft.remindAt(location).IsZero() {
		remindAt = draft.remindAt(location).In(location).Format("02.01.2006 15:04")
	}

	if draft.title != "" {
		title = draft.title
	}

	if draft.description != "" {
		description = draft.description
	}

	nextStep := draftNextStep(draft)

	notice := ""
	if draft.notice != "" {
		notice = "\n\n" + draft.notice
	}

	return fmt.Sprintf(
		"Создание напоминания\n\nДата и время: %s\nНазвание: %s\nОписание: %s\n%s\n%s%s",
		remindAt,
		title,
		description,
		repeat,
		nextStep,
		notice,
	)
}

func draftNextStep(draft reminderDraft) string {
	if draft.waitingFor == fieldTitle {
		return "Введите название напоминания сообщением."
	}

	if draft.waitingFor == fieldDescription {
		return "Введите описание напоминания сообщением."
	}

	if draft.screen == draftScreenDate {
		return "Выберите быстрый вариант даты или откройте календарь."
	}

	if draft.screen == draftScreenCalendar {
		return "Выберите дату в календаре."
	}

	if draft.screen == draftScreenTime {
		return "Выберите время напоминания."
	}

	if draft.screen == draftScreenRepeat {
		return "Выберите периодичность напоминания."
	}

	if draft.screen == draftScreenWeekdays {
		return "Выберите дни недели для повторения."
	}

	return "Выберите дату и время кнопкой ниже."
}

func reminderViewText(reminder database.ReminderView, location *time.Location) string {
	description := "Без описания"
	if reminder.Description != "" {
		description = reminder.Description
	}

	return fmt.Sprintf(
		"%s\n%s\n%s",
		reminder.RemindAt.In(location).Format("02.01.2006 15:04"),
		reminder.Title,
		description,
	)
}

func reminderListText(reminders []database.ReminderView) string {
	if len(reminders) == 0 {
		return "Активных напоминаний пока нет.\n\nСоздайте первое напоминание на основной клавиатуре."
	}

	return "Список напоминаний\n\nВыберите напоминание из списка."
}

func monthDayRows(month time.Time, now time.Time, selectedDates []time.Time) [][]telego.InlineKeyboardButton {
	firstDay := firstDayOfMonth(month)
	lastDay := firstDay.AddDate(0, 1, -1).Day()
	weekdayOffset := int(firstDay.Weekday()+6) % 7
	rows := make([][]telego.InlineKeyboardButton, 0, 6)

	var row []telego.InlineKeyboardButton
	for range weekdayOffset {
		row = append(row, tu.InlineKeyboardButton(" ").WithCallbackData(callbackNoop))
	}

	today := beginningOfDay(now)

	for day := 1; day <= lastDay; day++ {
		date := time.Date(firstDay.Year(), firstDay.Month(), day, 0, 0, 0, 0, month.Location())

		callback := dateCallback(date)
		if date.Before(today) {
			callback = callbackNoop
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
			row = append(row, tu.InlineKeyboardButton(" ").WithCallbackData(callbackNoop))
		}

		rows = append(rows, row)
	}

	return rows
}

func firstDayOfMonth(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
}

func dateCallback(value time.Time) string {
	return callbackDatePrefix + value.Format(time.DateOnly)
}

func monthCallback(value time.Time) string {
	return callbackMonthPrefix + value.Format("2006-01")
}

func timeSetCallback(hour, minute int) string {
	return fmt.Sprintf("%s%02d:%02d", callbackTimeSetPrefix, hour, minute)
}

func timeShiftCallback(minutes int) string {
	return callbackTimeShiftPrefix + strconv.Itoa(minutes)
}

func repeatCallback(value reminderRepeat) string {
	return callbackRepeatPrefix + string(value)
}

func weekdayCallback(mask int) string {
	return callbackWeekdayPrefix + strconv.Itoa(mask)
}

func repeatText(draft reminderDraft) string {
	switch draft.repeat {
	case reminderRepeatEveryDay:
		return "Повторяемость: каждый день\n"
	case reminderRepeatEveryWeek:
		return "Повторяемость: каждую неделю\n"
	case reminderRepeatEveryTwoDays:
		return "Повторяемость: каждые 2 дня\n"
	case reminderRepeatWeekdays:
		return "Повторяемость: по будням\n"
	case reminderRepeatCustomWeekdays:
		return fmt.Sprintf("Повторяемость: выбрано дней недели: %d\n", selectedWeekdayCount(draft.weekdaysMask))
	case reminderRepeatCustomDates:
		return fmt.Sprintf("Повторяемость: выбрано дат: %d\n", len(draft.customDates))
	default:
		return "Повторяемость: не задана\n"
	}
}

func selectedWeekdayCount(mask int) int {
	count := 0

	for _, dayMask := range []int{1, 2, 4, 8, 16, 32, 64} {
		if mask&dayMask != 0 {
			count++
		}
	}

	return count
}

func isDraftCustomDateSelected(date time.Time, selectedDates []time.Time) bool {
	for _, selected := range selectedDates {
		if selected.Year() == date.Year() && selected.Month() == date.Month() && selected.Day() == date.Day() {
			return true
		}
	}

	return false
}

func isFutureDraftTime(draft reminderDraft, hour int, minute int, now time.Time) bool {
	if draft.date.IsZero() {
		return true
	}

	remindAt := time.Date(
		draft.date.Year(),
		draft.date.Month(),
		draft.date.Day(),
		hour,
		minute,
		0,
		0,
		draft.date.Location(),
	)

	return remindAt.After(now)
}

func beginningOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
