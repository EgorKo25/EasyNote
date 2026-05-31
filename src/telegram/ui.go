package telegram

import (
	"fmt"
	"strconv"
	"strings"
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
	callbackDraftTitle       = "draft:title"
	callbackDraftDescription = "draft:description"
	callbackDraftDate        = "draft:date"
	callbackDraftCalendar    = "draft:calendar"
	callbackDraftBack        = "draft:back"
	callbackDraftSave        = "draft:save"
	callbackDraftCancel      = "draft:cancel"
	callbackDraftTimeText    = "draft:time_text"
	callbackDraftTimeDone    = "draft:time_done"
	callbackNoop             = "noop"
	callbackMainMenu         = "menu:main"
	callbackDatePrefix       = "date:"
	callbackMonthPrefix      = "month:"
	callbackTimeSetPrefix    = "time:set:"
	callbackTimeShiftPrefix  = "time:shift:"
	callbackDeletePrefix     = "reminder:delete:"
)
const (
	fieldTitle       draftField = "title"
	fieldDescription draftField = "description"
	fieldTime        draftField = "time"
)
const (
	draftScreenMain     draftScreen = "main"
	draftScreenDate     draftScreen = "date"
	draftScreenCalendar draftScreen = "calendar"
	draftScreenTime     draftScreen = "time"
)

const emptyFieldText = "не задано"

func mainKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(buttonCreateReminder)),
		tu.KeyboardRow(tu.KeyboardButton(buttonReminderList)),
	).WithResizeKeyboard()
}

func draftKeyboard(draft reminderDraft) *telego.InlineKeyboardMarkup {
	switch draft.screen {
	case draftScreenDate:
		return dateChoiceKeyboard()
	case draftScreenCalendar:
		return dateKeyboard(draft)
	case draftScreenTime:
		return timeKeyboard(draft)
	default:
		return draftMainKeyboard(draft)
	}
}

func draftMainKeyboard(draft reminderDraft) *telego.InlineKeyboardMarkup {
	timeButtonText := "Выбрать дату и время"
	if !draft.remindAt().IsZero() {
		timeButtonText = "Изменить дату и время"
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton(timeButtonText).WithCallbackData(callbackDraftDate)),
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

func dateChoiceKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Сегодня").WithCallbackData(dateCallback(time.Now())),
			tu.InlineKeyboardButton("Завтра").WithCallbackData(dateCallback(time.Now().AddDate(0, 0, 1))),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Календарь").WithCallbackData(callbackDraftCalendar)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(callbackDraftBack)),
	)
}

func dateKeyboard(draft reminderDraft) *telego.InlineKeyboardMarkup {
	month := draft.calendarMonth
	if month.IsZero() {
		month = firstDayOfMonth(time.Now())
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

	rows = append(rows, monthDayRows(month)...)
	rows = append(rows, tu.InlineKeyboardRow(tu.InlineKeyboardButton("Назад").WithCallbackData(callbackDraftDate)))

	return tu.InlineKeyboard(rows...)
}

func timeKeyboard(draft reminderDraft) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("09:00").WithCallbackData(timeSetCallback(9, 0)),
			tu.InlineKeyboardButton("12:00").WithCallbackData(timeSetCallback(12, 0)),
			tu.InlineKeyboardButton("18:00").WithCallbackData(timeSetCallback(18, 0)),
			tu.InlineKeyboardButton("21:00").WithCallbackData(timeSetCallback(21, 0)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("-1ч").WithCallbackData(timeShiftCallback(-60)),
			tu.InlineKeyboardButton(draft.selectedTimeText()).WithCallbackData(callbackNoop),
			tu.InlineKeyboardButton("+1ч").WithCallbackData(timeShiftCallback(60)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("-5м").WithCallbackData(timeShiftCallback(-5)),
			tu.InlineKeyboardButton("+5м").WithCallbackData(timeShiftCallback(5)),
		),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Ввести время").WithCallbackData(callbackDraftTimeText)),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Назад к дате").WithCallbackData(callbackDraftDate),
			tu.InlineKeyboardButton("Готово").WithCallbackData(callbackDraftTimeDone),
		),
	)
}

func reminderListKeyboard(reminders []database.ReminderView) *telego.InlineKeyboardMarkup {
	rows := make([][]telego.InlineKeyboardButton, 0, len(reminders)+1)
	for index, reminder := range reminders {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("Удалить #%d", index+1)).
				WithCallbackData(fmt.Sprintf("%s%d", callbackDeletePrefix, reminder.ID)),
		))
	}

	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Главное меню").WithCallbackData(callbackMainMenu),
	))

	return tu.InlineKeyboard(rows...)
}

func mainMenuText() string {
	return "Главное меню\n\nВыберите действие на клавиатуре ниже."
}

func draftText(draft reminderDraft) string {
	remindAt, title, description := emptyFieldText, emptyFieldText, emptyFieldText

	if !draft.remindAt().IsZero() {
		remindAt = draft.remindAt().Format("02.01.2006 15:04")
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
		"Создание напоминания\n\nДата и время: %s\nНазвание: %s\nОписание: %s\n\n%s%s",
		remindAt,
		title,
		description,
		nextStep,
		notice,
	)
}

func draftNextStep(draft reminderDraft) string {
	if draft.waitingFor == fieldTitle {
		return "Отправьте название одним сообщением."
	}

	if draft.waitingFor == fieldDescription {
		return "Отправьте описание одним сообщением."
	}

	if draft.waitingFor == fieldTime {
		return "Отправьте время одним сообщением в формате 09:30."
	}

	if draft.screen == draftScreenDate {
		return "Выберите быстрый вариант или откройте календарь."
	}

	if draft.screen == draftScreenCalendar {
		return "Выберите дату в календаре."
	}

	if draft.screen == draftScreenTime {
		return "Выберите готовое время или настройте его кнопками."
	}

	return "Выберите дату и время кнопкой ниже."
}

func reminderViewText(reminder database.ReminderView) string {
	description := "не задано"
	if reminder.Description != "" {
		description = reminder.Description
	}

	return fmt.Sprintf(
		"#%d: %s\n%s\n%s",
		reminder.ID,
		reminder.RemindAt.Format("02.01.2006 15:04"),
		reminder.Title,
		description,
	)
}

func reminderListText(reminders []database.ReminderView) string {
	if len(reminders) == 0 {
		return "Активных напоминаний пока нет.\n\nСоздайте первое напоминание на основной клавиатуре."
	}

	var builder strings.Builder
	builder.WriteString("Активные напоминания\n")

	for index, reminder := range reminders {
		description := reminder.Description
		if description == "" {
			description = "без описания"
		}

		builder.WriteString(fmt.Sprintf(
			"\n%d. %s\n%s\n%s\n",
			index+1,
			reminder.RemindAt.Format("02.01.2006 15:04"),
			reminder.Title,
			description,
		))
	}

	return builder.String()
}

func monthDayRows(month time.Time) [][]telego.InlineKeyboardButton {
	firstDay := firstDayOfMonth(month)
	lastDay := firstDay.AddDate(0, 1, -1).Day()
	weekdayOffset := int(firstDay.Weekday()+6) % 7
	rows := make([][]telego.InlineKeyboardButton, 0, 6)

	var row []telego.InlineKeyboardButton
	for range weekdayOffset {
		row = append(row, tu.InlineKeyboardButton(" ").WithCallbackData(callbackNoop))
	}

	for day := 1; day <= lastDay; day++ {
		date := time.Date(firstDay.Year(), firstDay.Month(), day, 0, 0, 0, 0, time.Local)
		row = append(row, tu.InlineKeyboardButton(strconv.Itoa(day)).WithCallbackData(dateCallback(date)))

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
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.Local)
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
