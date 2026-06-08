package ui

const (
	ButtonStart           = "Старт"
	ButtonCreateReminder  = "Создать напоминание"
	ButtonReminderList    = "Список напоминаний"
	ButtonReminderHistory = "Прошедшие напоминания"
	ButtonSettings        = "Настройки"
)

const (
	CallbackDraftTitle              = "draft:title"
	CallbackDraftDescription        = "draft:description"
	CallbackDraftDate               = "draft:date"
	CallbackDraftCalendar           = "draft:calendar"
	CallbackDraftRepeat             = "draft:repeat"
	CallbackDraftBack               = "draft:back"
	CallbackDraftSave               = "draft:save"
	CallbackDraftCancel             = "draft:cancel"
	CallbackDraftTimeDone           = "draft:time_done"
	CallbackDraftCustomDone         = "draft:custom_dates_done"
	CallbackDraftWeekdaysDone       = "draft:weekdays_done"
	CallbackDraftIntervalDone       = "draft:interval_done"
	CallbackSettings                = "settings:open"
	CallbackSettingsLanguage        = "settings:language"
	CallbackSettingsTimezone        = "settings:timezone"
	CallbackSettingsIntrusive       = "settings:intrusive"
	CallbackSettingsIntrusiveOn     = "settings:intrusive:on"
	CallbackSettingsIntrusiveOff    = "settings:intrusive:off"
	CallbackReminderDone            = "reminder:done:"
	CallbackReminderCancelDelivered = "reminder:cancel_delivered:"
	CallbackReminderHistory         = "reminder:history"
	CallbackNoop                    = "noop"
	CallbackMainMenu                = "menu:main"
	CallbackCreateReminder          = "reminder:create"
	CallbackReminderList            = "reminder:list"
	CallbackReminderBackList        = "reminder:back_list"
	CallbackDatePrefix              = "date:"
	CallbackMonthPrefix             = "month:"
	CallbackTimeSetPrefix           = "time:set:"
	CallbackTimeShiftPrefix         = "time:shift:"
	CallbackRepeatPrefix            = "repeat:"
	CallbackWeekdayPrefix           = "weekday:"
	CallbackViewPrefix              = "reminder:view:"
	CallbackDeletePrefix            = "reminder:delete:"
)

const (
	FieldTitle             Field = "title"
	FieldDescription       Field = "description"
	FieldTimezoneCity      Field = "timezone_city"
	FieldIntrusiveInterval Field = "intrusive_interval"
	FieldRepeatInterval    Field = "repeat_interval"
)

const (
	ScreenMain     Screen = "main"
	ScreenDate     Screen = "date"
	ScreenCalendar Screen = "calendar"
	ScreenTime     Screen = "time"
	ScreenRepeat   Screen = "repeat"
	ScreenWeekdays Screen = "weekdays"
	ScreenInterval Screen = "interval"
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
	RepeatCustomInterval Repeat = "custom_interval"
)
