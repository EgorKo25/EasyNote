package ui

import (
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

func MainMenuText() string {
	return "Главное меню\n\nВыберите действие на клавиатуре ниже."
}

func DraftText(draft Draft, location *time.Location) string {
	remindAt, title, description := emptyFieldText, emptyFieldText, emptyFieldText
	repeat := RepeatText(draft)

	if !draft.RemindAt(location).IsZero() {
		remindAt = draft.RemindAt(location).In(location).Format("02.01.2006 15:04")
	}

	if draft.Title != "" {
		title = draft.Title
	}

	if draft.Description != "" {
		description = draft.Description
	}

	nextStep := draftNextStep(draft)

	notice := ""
	if draft.Notice != "" {
		notice = "\n\n" + draft.Notice
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

func draftNextStep(draft Draft) string {
	if draft.WaitingFor == FieldTitle {
		return "Введите название напоминания сообщением."
	}

	if draft.WaitingFor == FieldDescription {
		return "Введите описание напоминания сообщением."
	}

	if draft.Screen == ScreenDate {
		return "Выберите быстрый вариант даты или откройте календарь."
	}

	if draft.Screen == ScreenCalendar {
		return "Выберите дату в календаре."
	}

	if draft.Screen == ScreenTime {
		return "Выберите время напоминания."
	}

	if draft.Screen == ScreenRepeat {
		return "Выберите периодичность напоминания."
	}

	if draft.Screen == ScreenWeekdays {
		return "Выберите дни недели для повторения."
	}

	return "Выберите дату и время кнопкой ниже."
}

func ReminderViewText(reminder database.ReminderView, location *time.Location) string {
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

func ReminderListText(reminders []database.ReminderView) string {
	if len(reminders) == 0 {
		return "Активных напоминаний пока нет.\n\nСоздайте первое напоминание на основной клавиатуре."
	}

	return "Список напоминаний\n\nВыберите напоминание из списка."
}

func RepeatText(draft Draft) string {
	switch draft.Repeat {
	case RepeatEveryDay:
		return "Повторяемость: каждый день\n"
	case RepeatEveryWeek:
		return "Повторяемость: каждую неделю\n"
	case RepeatEveryTwoDays:
		return "Повторяемость: каждые 2 дня\n"
	case RepeatWeekdays:
		return "Повторяемость: по будням\n"
	case RepeatCustomWeekdays:
		return fmt.Sprintf("Повторяемость: выбрано дней недели: %d\n", SelectedWeekdayCount(draft.WeekdaysMask))
	case RepeatCustomDates:
		return fmt.Sprintf("Повторяемость: выбрано дат: %d\n", len(draft.CustomDates))
	default:
		return "Повторяемость: не задана\n"
	}
}

func SelectedWeekdayCount(mask int) int {
	count := 0

	for _, dayMask := range []int{1, 2, 4, 8, 16, 32, 64} {
		if mask&dayMask != 0 {
			count++
		}
	}

	return count
}
