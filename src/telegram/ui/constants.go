package ui

const (
	ButtonStart          = "Старт"
	ButtonCreateReminder = "Создать напоминание"
	ButtonReminderList   = "Список напоминаний"
)

const (
	CallbackDraftTitle        = "draft:title"
	CallbackDraftDescription  = "draft:description"
	CallbackDraftDate         = "draft:date"
	CallbackDraftCalendar     = "draft:calendar"
	CallbackDraftRepeat       = "draft:repeat"
	CallbackDraftBack         = "draft:back"
	CallbackDraftSave         = "draft:save"
	CallbackDraftCancel       = "draft:cancel"
	CallbackDraftTimeDone     = "draft:time_done"
	CallbackDraftCustomDone   = "draft:custom_dates_done"
	CallbackDraftWeekdaysDone = "draft:weekdays_done"
	CallbackNoop              = "noop"
	CallbackMainMenu          = "menu:main"
	CallbackCreateReminder    = "reminder:create"
	CallbackReminderList      = "reminder:list"
	CallbackReminderBackList  = "reminder:back_list"
	CallbackDatePrefix        = "date:"
	CallbackMonthPrefix       = "month:"
	CallbackTimeSetPrefix     = "time:set:"
	CallbackTimeShiftPrefix   = "time:shift:"
	CallbackRepeatPrefix      = "repeat:"
	CallbackWeekdayPrefix     = "weekday:"
	CallbackViewPrefix        = "reminder:view:"
	CallbackDeletePrefix      = "reminder:delete:"
)

const (
	FieldTitle       Field = "title"
	FieldDescription Field = "description"
)

const (
	ScreenMain     Screen = "main"
	ScreenDate     Screen = "date"
	ScreenCalendar Screen = "calendar"
	ScreenTime     Screen = "time"
	ScreenRepeat   Screen = "repeat"
	ScreenWeekdays Screen = "weekdays"
)

const emptyFieldText = "не задано"

type Field string
type Screen string
type Repeat string

const (
	RepeatNone           Repeat = ""
	RepeatEveryDay       Repeat = "every_day"
	RepeatEveryWeek      Repeat = "every_week"
	RepeatEveryTwoDays   Repeat = "every_two_days"
	RepeatWeekdays       Repeat = "weekdays"
	RepeatCustomWeekdays Repeat = "custom_weekdays"
	RepeatCustomDates    Repeat = "custom_dates"
)
