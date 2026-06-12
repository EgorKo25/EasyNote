package ui

import (
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

func MainMenuText() string {
	return "Главное меню\n\nВыберите действие на клавиатуре."
}

func RemindersText() string {
	return "Напоминания\n\nВыберите список напоминаний."
}

func CreateText() string {
	return "Создать\n\nВыберите тип."
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
		return "Выберите регулярность напоминания."
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

func ReminderHistoryText(reminders []database.ReminderView) string {
	if len(reminders) == 0 {
		return "Прошедших напоминаний пока нет."
	}

	return "Прошедшие напоминания\n\nВыберите напоминание из списка."
}

func SettingsText(settings database.UserSettings) string {
	return fmt.Sprintf(
		"Настройки\n\nЯзык: русский\nЧасовой пояс: %s\nНастойчивые напоминания: %s\nИнтервал: %s",
		settings.Timezone,
		intrusiveEnabledText(settings.IntrusiveEnabled),
		settings.IntrusiveInterval,
	)
}

func IntrusiveSettingsText(settings database.UserSettings) string {
	return fmt.Sprintf(
		"Настойчивые напоминания\n\nСостояние: %s\nИнтервал: %s",
		intrusiveEnabledText(settings.IntrusiveEnabled),
		settings.IntrusiveInterval,
	)
}

func IntrusiveIntervalText(interval time.Duration) string {
	return fmt.Sprintf("Интервал настойчивых напоминаний\n\nТекущее значение: %s", interval)
}

func ReminderStatusText(status database.ReminderStatus) string {
	switch status {
	case database.ReminderStatusCompleted:
		return "Выполнено"
	case database.ReminderStatusCanceled:
		return "Отменено"
	case database.ReminderStatusSkipped:
		return "Пропущено"
	case database.ReminderStatusSent:
		return "Отправлено"
	case database.ReminderStatusFailed:
		return "Ошибка отправки"
	default:
		return string(status)
	}
}

func intrusiveEnabledText(enabled bool) string {
	if enabled {
		return "включены"
	}

	return "выключены"
}

func RepeatText(draft Draft) string {
	switch draft.Repeat {
	case RepeatEveryDay:
		return "Повторять: каждый день\n"
	case RepeatEveryWeek:
		return "Повторять: каждую неделю\n"
	case RepeatEveryTwoDays:
		return "Повторять: каждые 2 дня\n"
	case RepeatWeekdays:
		return "Повторять: по будням\n"
	case RepeatCustomWeekdays:
		return fmt.Sprintf("Повторять: выбрано дней недели: %d\n", SelectedWeekdayCount(draft.WeekdaysMask))
	case RepeatCustomDates:
		return fmt.Sprintf("Повторять: выбрано дат: %d\n", len(draft.CustomDates))
	case RepeatCustomInterval:
		return fmt.Sprintf("Повторять: каждые %d дн.\n", draft.IntervalDays)
	default:
		return "Повторять: нет\n"
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
