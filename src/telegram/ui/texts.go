package ui

import (
	"fmt"
	"html"
	"strings"
	"time"

	telegrammodel "github.com/EgorKo25/EasyNote/src/telegram/model"
)

func MainMenuText(stats telegrammodel.ReminderStats) string {
	return fmt.Sprintf(
		"⠀⠀⠀⠀⠀⠀⠀⠀<b>Главное меню</b>\n\n"+
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

func HolidaysText(holidays []telegrammodel.HolidayView) string {
	if len(holidays) == 0 {
		return "<b>Праздники</b>\n\n<i>Сохраненных праздников пока нет.</i>"
	}

	return "<b>Праздники</b>\n\n<i>Выберите праздник из списка.</i>"
}

func ShoppingListsText(lists []telegrammodel.ShoppingListView) string {
	if len(lists) == 0 {
		return "<b>Списки покупок</b>\n\n<i>Сохраненных списков пока нет.</i>"
	}

	return "<b>Списки покупок</b>\n\n<i>Выберите список.</i>"
}

func CreateText() string {
	return "<b>Создать</b>\n\n<i>Выберите тип.</i>"
}

func DraftText(draft Draft, location *time.Location) string {
	switch draft.Kind {
	case DraftKindHoliday:
		return HolidayDraftText(draft, location)
	case DraftKindShopping:
		return ShoppingDraftText(draft)
	default:
		return ReminderDraftText(draft, location)
	}
}

func ReminderDraftText(draft Draft, location *time.Location) string {
	dateText := ""
	title := html.EscapeString(draft.Title)
	description := html.EscapeString(draft.Description)
	repeat := strings.TrimPrefix(RepeatText(draft), "Повторять: ")

	remindAt := draft.RemindAt(location)
	if remindAt.IsZero() && len(draft.CustomDates) > 0 {
		remindAt = time.Date(
			draft.CustomDates[0].Year(),
			draft.CustomDates[0].Month(),
			draft.CustomDates[0].Day(),
			draft.Hour,
			draft.Minute,
			0,
			0,
			location,
		)
	}

	if !remindAt.IsZero() {
		remindAt = remindAt.In(location)
		dateText = fmt.Sprintf(
			"%s, %s %s",
			remindAt.Format("02.01.2006"),
			weekdayText(remindAt),
			remindAt.Format("15:04"),
		)
	}

	notice := ""
	if draft.Notice != "" {
		notice = "\n\n<i>" + html.EscapeString(draft.Notice) + "</i>"
	}

	return fmt.Sprintf(
		"<b>Название:</b>\n%s\n\n"+
			"<b>Описание:</b>\n%s\n\n"+
			"<b>Дата начала напоминаний:</b>\n%s\n\n"+
			"<b>Повторять:</b>\n%s\n\n"+
			"<i>%s</i>%s",
		title,
		description,
		dateText,
		repeat,
		draftNextStep(draft, location),
		notice,
	)
}

func HolidayDraftText(draft Draft, location *time.Location) string {
	holidayName := html.EscapeString(draft.Title)
	personName := html.EscapeString(draft.Description)
	dateText := ""
	timeText := ""

	remindAt := draft.RemindAt(location)
	if !remindAt.IsZero() {
		dateText = remindAt.In(location).Format("02.01.2006")
		timeText = remindAt.In(location).Format("15:04")
	}

	notice := ""
	if draft.Notice != "" {
		notice = "\n\n<i>" + html.EscapeString(draft.Notice) + "</i>"
	}

	return fmt.Sprintf(
		"Поздравьте <b>%s</b> с <b>%s</b>.\n\n"+
			"<b>Дата:</b> %s\n\n"+
			"<b>Время:</b> %s%s",
		personName,
		holidayName,
		dateText,
		timeText,
		notice,
	)
}

func ShoppingDraftText(draft Draft) string {
	items := make([]string, 0, len(draft.ShoppingItems))
	for index, item := range draft.ShoppingItems {
		items = append(items, fmt.Sprintf("<b>%d.</b> %s", index+1, html.EscapeString(item.Text)))
	}

	if len(items) == 0 {
		items = append(items, "<b>1.</b> ...")
	}

	notice := ""
	if draft.Notice != "" {
		notice = "\n\n<i>" + html.EscapeString(draft.Notice) + "</i>"
	}

	return "<b>Необходимо купить:</b>\n\n" + strings.Join(items, "\n") + notice
}

func draftNextStep(draft Draft, location *time.Location) string {
	if prompt := draftWaitingPrompt(draft.WaitingFor); prompt != "" {
		return prompt
	}

	if prompt := draftScreenPrompt(draft.Screen); prompt != "" {
		return prompt
	}

	hasTitle := draft.Title != ""
	hasTime := !draft.RemindAt(location).IsZero()

	return draftRequiredFieldsPrompt(hasTitle, hasTime)
}

func draftWaitingPrompt(field Field) string {
	switch field {
	case FieldTitle:
		return "Введите название напоминания сообщением."
	case FieldDescription:
		return "Введите описание напоминания сообщением."
	case FieldShoppingItem:
		return "Введите пункт списка сообщением."
	default:
		return ""
	}
}

func draftScreenPrompt(screen Screen) string {
	switch screen {
	case ScreenDate:
		return "Выберите быстрый вариант даты или откройте календарь."
	case ScreenCalendar:
		return "Выберите дату в календаре."
	case ScreenTime:
		return "Введите время напоминания сообщением в формате HH:MM."
	case ScreenRepeat:
		return "Выберите регулярность напоминания."
	case ScreenWeekdays:
		return "Выберите дни недели для повторения."
	default:
		return ""
	}
}

func draftRequiredFieldsPrompt(hasTitle bool, hasTime bool) string {
	switch {
	case hasTitle && !hasTime:
		return "Выберите дату и введите время напоминания."
	case !hasTitle && hasTime:
		return "Введите название напоминания."
	case !hasTitle && !hasTime:
		return "Введите название и время напоминания."
	default:
		return "Проверьте шаблон и сохраните напоминание."
	}
}

func ReminderViewText(reminder telegrammodel.ReminderView, location *time.Location) string {
	description := "Без описания"
	if reminder.Description != "" {
		description = html.EscapeString(reminder.Description)
	}

	return fmt.Sprintf(
		"<b>%s</b>\n\n%s\n\n%s",
		html.EscapeString(reminder.Title),
		description,
		reminder.RemindAt.In(location).Format("02.01.2006 15:04"),
	)
}

func ShoppingListViewText(list telegrammodel.ShoppingListView) string {
	items := make([]string, 0, len(list.Items))
	for index, item := range list.Items {
		text := html.EscapeString(item.Text)
		if item.Done {
			text = "<s>" + text + "</s>"
		}

		items = append(items, fmt.Sprintf("<b>%d.</b> %s", index+1, text))
	}

	return "<b>Необходимо купить:</b>\n\n" + strings.Join(items, "\n")
}

func ReminderListText(reminders []telegrammodel.ReminderView) string {
	if len(reminders) == 0 {
		return "<b>Активные напоминания</b>\n\n<i>Активных напоминаний пока нет.</i>"
	}

	return "<b>Список напоминаний</b>\n\n<i>Выберите напоминание из списка.</i>"
}

func ReminderHistoryText(reminders []telegrammodel.ReminderView) string {
	if len(reminders) == 0 {
		return "<b>Прошедшие напоминания</b>\n\n<i>Прошедших напоминаний пока нет.</i>"
	}

	return "<b>Прошедшие напоминания</b>\n\n<i>Выберите напоминание из списка.</i>"
}

func SettingsText(settings telegrammodel.UserSettings) string {
	return fmt.Sprintf(
		"<b>Настройки</b>\n\nЯзык: русский\nЧасовой пояс: %s\nНастойчивые напоминания: %s\nИнтервал: %s",
		html.EscapeString(settings.Timezone),
		intrusiveEnabledText(settings.IntrusiveEnabled),
		IntrusiveIntervalValueText(settings.IntrusiveInterval),
	)
}

func IntrusiveSettingsText(settings telegrammodel.UserSettings) string {
	return fmt.Sprintf(
		"<b>Настойчивые напоминания</b>\n\nСостояние: %s\nИнтервал: %s\n\n"+
			"<i>Вы можете ввести желаемый интервал напоминания с клавиатуры. "+
			"Лучше использовать формат 1h30m. Просто отправьте его в чат.</i>",
		intrusiveEnabledText(settings.IntrusiveEnabled),
		IntrusiveIntervalValueText(settings.IntrusiveInterval),
	)
}

func IntrusiveIntervalValueText(interval time.Duration) string {
	minutes := int(interval / time.Minute)
	if minutes < 60 {
		if minutes == 1 {
			return "каждую минуту"
		}

		return fmt.Sprintf("каждые %d %s", minutes, russianPlural(minutes, "минута", "минуты", "минут"))
	}

	hours := minutes / 60
	restMinutes := minutes % 60

	if restMinutes == 0 {
		return fmt.Sprintf("раз в %d %s", hours, russianPlural(hours, "час", "часа", "часов"))
	}

	return fmt.Sprintf(
		"раз в %d %s %d %s",
		hours,
		russianPlural(hours, "час", "часа", "часов"),
		restMinutes,
		russianPlural(restMinutes, "минута", "минуты", "минут"),
	)
}

func russianPlural(value int, one string, few string, many string) string {
	if value%100 >= 11 && value%100 <= 14 {
		return many
	}

	switch value % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

func ReminderStatusText(status telegrammodel.ReminderStatus) string {
	switch status {
	case telegrammodel.ReminderStatusCompleted:
		return "Выполнено"
	case telegrammodel.ReminderStatusCanceled:
		return "Отменено"
	case telegrammodel.ReminderStatusSkipped:
		return "Пропущено"
	case telegrammodel.ReminderStatusSent:
		return "Отправлено"
	case telegrammodel.ReminderStatusFailed:
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
