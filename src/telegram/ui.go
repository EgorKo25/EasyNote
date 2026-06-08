package telegram

import (
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	telegramui "github.com/EgorKo25/EasyNote/src/telegram/ui"
	"github.com/mymmrac/telego"
)

const (
	buttonStart           = telegramui.ButtonStart
	buttonCreateReminder  = telegramui.ButtonCreateReminder
	buttonReminders       = telegramui.ButtonReminders
	buttonReminderList    = telegramui.ButtonReminderList
	buttonReminderHistory = telegramui.ButtonReminderHistory
	buttonSettings        = telegramui.ButtonSettings
)

const (
	callbackDraftTitle              = telegramui.CallbackDraftTitle
	callbackDraftDescription        = telegramui.CallbackDraftDescription
	callbackDraftDate               = telegramui.CallbackDraftDate
	callbackDraftCalendar           = telegramui.CallbackDraftCalendar
	callbackDraftRepeat             = telegramui.CallbackDraftRepeat
	callbackDraftBack               = telegramui.CallbackDraftBack
	callbackDraftSave               = telegramui.CallbackDraftSave
	callbackDraftCancel             = telegramui.CallbackDraftCancel
	callbackDraftTimeDone           = telegramui.CallbackDraftTimeDone
	callbackDraftCustomDone         = telegramui.CallbackDraftCustomDone
	callbackDraftWeekdaysDone       = telegramui.CallbackDraftWeekdaysDone
	callbackDraftIntervalDone       = telegramui.CallbackDraftIntervalDone
	callbackSettings                = telegramui.CallbackSettings
	callbackSettingsLanguage        = telegramui.CallbackSettingsLanguage
	callbackSettingsTimezone        = telegramui.CallbackSettingsTimezone
	callbackSettingsIntrusive       = telegramui.CallbackSettingsIntrusive
	callbackSettingsIntrusiveOn     = telegramui.CallbackSettingsIntrusiveOn
	callbackSettingsIntrusiveOff    = telegramui.CallbackSettingsIntrusiveOff
	callbackReminderDone            = telegramui.CallbackReminderDone
	callbackReminderCancelDelivered = telegramui.CallbackReminderCancelDelivered
	callbackReminderHistory         = telegramui.CallbackReminderHistory
	callbackNoop                    = telegramui.CallbackNoop
	callbackMainMenu                = telegramui.CallbackMainMenu
	callbackCreateReminder          = telegramui.CallbackCreateReminder
	callbackReminderList            = telegramui.CallbackReminderList
	callbackReminderBackList        = telegramui.CallbackReminderBackList
	callbackDatePrefix              = telegramui.CallbackDatePrefix
	callbackMonthPrefix             = telegramui.CallbackMonthPrefix
	callbackTimeSetPrefix           = telegramui.CallbackTimeSetPrefix
	callbackTimeShiftPrefix         = telegramui.CallbackTimeShiftPrefix
	callbackRepeatPrefix            = telegramui.CallbackRepeatPrefix
	callbackWeekdayPrefix           = telegramui.CallbackWeekdayPrefix
	callbackViewPrefix              = telegramui.CallbackViewPrefix
	callbackDeletePrefix            = telegramui.CallbackDeletePrefix
)

const (
	fieldTitle             draftField = telegramui.FieldTitle
	fieldDescription       draftField = telegramui.FieldDescription
	fieldTimezoneCity      draftField = telegramui.FieldTimezoneCity
	fieldIntrusiveInterval draftField = telegramui.FieldIntrusiveInterval
	fieldRepeatInterval    draftField = telegramui.FieldRepeatInterval
)

const (
	draftScreenMain     draftScreen = telegramui.ScreenMain
	draftScreenDate     draftScreen = telegramui.ScreenDate
	draftScreenCalendar draftScreen = telegramui.ScreenCalendar
	draftScreenTime     draftScreen = telegramui.ScreenTime
	draftScreenRepeat   draftScreen = telegramui.ScreenRepeat
	draftScreenWeekdays draftScreen = telegramui.ScreenWeekdays
	draftScreenInterval draftScreen = telegramui.ScreenInterval
)

const (
	reminderRepeatNone           reminderRepeat = telegramui.RepeatNone
	reminderRepeatEveryDay       reminderRepeat = telegramui.RepeatEveryDay
	reminderRepeatEveryWeek      reminderRepeat = telegramui.RepeatEveryWeek
	reminderRepeatEveryTwoDays   reminderRepeat = telegramui.RepeatEveryTwoDays
	reminderRepeatWeekdays       reminderRepeat = telegramui.RepeatWeekdays
	reminderRepeatCustomWeekdays reminderRepeat = telegramui.RepeatCustomWeekdays
	reminderRepeatCustomDates    reminderRepeat = telegramui.RepeatCustomDates
	reminderRepeatCustomInterval reminderRepeat = telegramui.RepeatCustomInterval
)

type reminderRepeat = telegramui.Repeat
type draftField = telegramui.Field
type draftScreen = telegramui.Screen

func mainKeyboard() *telego.ReplyKeyboardMarkup {
	return telegramui.MainKeyboard()
}

func remindersKeyboard() *telego.InlineKeyboardMarkup {
	return telegramui.RemindersKeyboard()
}

func draftKeyboard(draft reminderDraft, now time.Time) *telego.InlineKeyboardMarkup {
	return telegramui.DraftKeyboard(uiDraft(draft), now)
}

func reminderListKeyboard(reminders []database.ReminderView) *telego.InlineKeyboardMarkup {
	return telegramui.ReminderListKeyboard(reminders)
}

func reminderDetailsKeyboard(reminder database.ReminderView) *telego.InlineKeyboardMarkup {
	return telegramui.ReminderDetailsKeyboard(reminder)
}

func reminderCreatedKeyboard() *telego.InlineKeyboardMarkup {
	return telegramui.ReminderCreatedKeyboard()
}

func settingsKeyboard() *telego.InlineKeyboardMarkup {
	return telegramui.SettingsKeyboard()
}

func intrusiveSettingsKeyboard() *telego.InlineKeyboardMarkup {
	return telegramui.IntrusiveSettingsKeyboard()
}

func mainMenuText() string {
	return telegramui.MainMenuText()
}

func remindersText() string {
	return telegramui.RemindersText()
}

func draftText(draft reminderDraft, location *time.Location) string {
	return telegramui.DraftText(uiDraft(draft), location)
}

func reminderViewText(reminder database.ReminderView, location *time.Location) string {
	return telegramui.ReminderViewText(reminder, location)
}

func reminderListText(reminders []database.ReminderView) string {
	return telegramui.ReminderListText(reminders)
}

func reminderHistoryText(reminders []database.ReminderView, location *time.Location) string {
	return telegramui.ReminderHistoryText(reminders, location)
}

func settingsText(settings database.UserSettings) string {
	return telegramui.SettingsText(settings)
}

func intrusiveSettingsText(settings database.UserSettings) string {
	return telegramui.IntrusiveSettingsText(settings)
}

func firstDayOfMonth(value time.Time) time.Time {
	return telegramui.FirstDayOfMonth(value)
}

func beginningOfDay(value time.Time) time.Time {
	return telegramui.BeginningOfDay(value)
}

func uiDraft(draft reminderDraft) telegramui.Draft {
	return telegramui.Draft{
		Title:         draft.title,
		Description:   draft.description,
		Notice:        draft.notice,
		Date:          draft.date,
		Hour:          draft.hour,
		Minute:        draft.minute,
		Repeat:        draft.repeat,
		CustomDates:   draft.customDates,
		WeekdaysMask:  draft.weekdaysMask,
		IntervalDays:  draft.intervalDays,
		Screen:        draft.screen,
		CalendarMonth: draft.calendarMonth,
		WaitingFor:    draft.waitingFor,
	}
}
