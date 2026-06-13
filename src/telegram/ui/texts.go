package ui

import (
	"fmt"
	"html"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

func MainMenuText(stats database.ReminderStats) string {
	return fmt.Sprintf(
		"⠀⠀⠀⠀<b>Главное меню</b>\n\n"+
			"✅ Задачи выполнены: %d\n"+
			"⏳ Задачи ожидают выполнения: %d\n"+
			"❗ Задачи пропущены: %d",
		stats.Completed,
		stats.Pending,
		stats.Skipped,
	)
}

func RemindersText() string {
	return "<b>Напоминания</b>\n\n<i>Выберите список напоминаний.</i>"
}

func CreateText() string {
	return "<b>Создать</b>\n\n<i>Выберите тип.</i>"
}

func DraftText(draft Draft, location *time.Location) string {
	dateText := "Сегодня"
	title := "Название"
	description := "Здесь может быть описание вашего напоминания."
	repeat := RepeatText(draft)

	if !draft.RemindAt(location).IsZero() {
		remindAt := draft.RemindAt(location).In(location)
		dateText = fmt.Sprintf(
			"%s, %s %s",
			remindAt.Format("02.01.2006"),
			weekdayText(remindAt),
			remindAt.Format("15:04"),
		)
	}

	if draft.Title != "" {
		title = html.EscapeString(draft.Title)
	}

	if draft.Description != "" {
		description = html.EscapeString(draft.Description)
	}

	nextStep := draftNextStep(draft)

	notice := ""
	if draft.Notice != "" {
		notice = "\n\n<i>" + html.EscapeString(draft.Notice) + "</i>"
	}

	return fmt.Sprintf(
		"<i>Шаблон напоминания</i>\n\n"+
			"⠀⠀⠀⠀<b>%s</b>\n\nОписание: %s\n%s\n\nДата начала напоминания:\n⠀⠀⠀⠀%s\n\n<i>%s</i>%s",
		title,
		description,
		repeat,
		dateText,
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
		description = html.EscapeString(reminder.Description)
	}

	return fmt.Sprintf(
		"%s\n<b>%s</b>\n%s",
		reminder.RemindAt.In(location).Format("02.01.2006 15:04"),
		html.EscapeString(reminder.Title),
		description,
	)
}

func ReminderListText(reminders []database.ReminderView) string {
	if len(reminders) == 0 {
		return "<b>Активные напоминания</b>\n\n<i>Активных напоминаний пока нет.</i>"
	}

	return "<b>Список напоминаний</b>\n\n<i>Выберите напоминание из списка.</i>"
}

func ReminderHistoryText(reminders []database.ReminderView) string {
	if len(reminders) == 0 {
		return "<b>Прошедшие напоминания</b>\n\n<i>Прошедших напоминаний пока нет.</i>"
	}

	return "<b>Прошедшие напоминания</b>\n\n<i>Выберите напоминание из списка.</i>"
}

func SettingsText(settings database.UserSettings) string {
	return fmt.Sprintf(
		"<b>Настройки</b>\n\nЯзык: русский\nЧасовой пояс: %s\nНастойчивые напоминания: %s\nИнтервал: %s",
		html.EscapeString(settings.Timezone),
		intrusiveEnabledText(settings.IntrusiveEnabled),
		settings.IntrusiveInterval,
	)
}

func IntrusiveSettingsText(settings database.UserSettings) string {
	return fmt.Sprintf(
		"<b>Настойчивые напоминания</b>\n\nСостояние: %s\nИнтервал: %s\n\n"+
			"<i>Вы можете ввести желаемый интервал напоминания с клавиатуры. Просто отправьте его в чат.</i>",
		intrusiveEnabledText(settings.IntrusiveEnabled),
		settings.IntrusiveInterval,
	)
}

func IntrusiveIntervalText(interval time.Duration) string {
	return fmt.Sprintf(
		"<b>Интервал настойчивых напоминаний</b>\n\nТекущее значение: %s\n\n<i>Выберите шаг или задайте минуты вручную.</i>",
		interval,
	)
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

func weekdayText(value time.Time) string {
	switch value.Weekday() {
	case time.Monday:
		return "Пн"
	case time.Tuesday:
		return "Вт"
	case time.Wednesday:
		return "Ср"
	case time.Thursday:
		return "Чт"
	case time.Friday:
		return "Пт"
	case time.Saturday:
		return "Сб"
	default:
		return "Вс"
	}
}

func RepeatText(draft Draft) string {
	switch draft.Repeat {
	case RepeatEveryDay:
		return "Повторять: каждый день"
	case RepeatEveryWeek:
		return "Повторять: каждую неделю"
	case RepeatEveryTwoDays:
		return "Повторять: каждые 2 дня"
	case RepeatWeekdays:
		return "Повторять: по будням"
	case RepeatCustomWeekdays:
		return fmt.Sprintf("Повторять: выбрано дней недели: %d", SelectedWeekdayCount(draft.WeekdaysMask))
	case RepeatCustomDates:
		return fmt.Sprintf("Повторять: выбрано дат: %d", len(draft.CustomDates))
	case RepeatCustomInterval:
		return fmt.Sprintf("Повторять: каждые %d дн.", draft.IntervalDays)
	default:
		return "Повторять: нет"
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
