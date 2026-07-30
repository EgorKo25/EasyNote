package apphttp

import "errors"

var ErrTelegramChatIDMissing = errors.New("telegram chat id is missing")
var ErrTelegramChatIDInvalid = errors.New("telegram chat id is invalid")

var ErrInvalidRequest = errors.New("invalid request")
var ErrInvalidID = errors.New("invalid id")
var ErrInvalidItemID = errors.New("invalid item id")
var ErrInvalidOffset = errors.New("invalid offset")
var ErrInvalidLimit = errors.New("invalid limit")
var ErrReminderStatusesEmpty = errors.New("reminder statuses are empty")
var ErrReminderStatusInvalid = errors.New("reminder status is invalid")
var ErrQueryContentTypeMissing = errors.New("query content type is missing")
var ErrQueryContentTypeUnsupported = errors.New("query content type is unsupported")

var ErrReminderRepeatIntervalDaysEmpty = errors.New("reminder repeat interval_days is empty")
var ErrReminderRepeatIntervalDaysInvalid = errors.New("reminder repeat interval_days is invalid")
var ErrReminderRepeatWeekdaysMaskEmpty = errors.New("reminder repeat weekdays_mask is empty")
var ErrReminderRepeatCustomDatesEmpty = errors.New("reminder repeat dates is empty")
var ErrReminderRepeatCustomTimeEmpty = errors.New("reminder repeat custom time is empty")
var ErrReminderRepeatCustomTimeInvalid = errors.New("reminder repeat custom time is invalid")
var ErrReminderRepeatTypeUnsupported = errors.New("unsupported reminder repeat type")
var ErrReminderRepeatDateInvalid = errors.New("reminder repeat date is invalid")
var ErrReminderRepeatTimezoneInvalid = errors.New("reminder repeat timezone is invalid")

var ErrHolidayNameEmpty = errors.New("holiday name is empty")
var ErrHolidayRecipientNameEmpty = errors.New("holiday recipient name is empty")
var ErrHolidayDateInvalid = errors.New("holiday date is invalid")
var ErrHolidayRemindAtEmpty = errors.New("holiday remind_at is empty")

var ErrShoppingListItemsEmpty = errors.New("shopping list items is empty")
var ErrShoppingListItemTextEmpty = errors.New("shopping list item text is empty")

var ErrSettingsTimezoneInvalid = errors.New("settings timezone is invalid")
var ErrIntrusiveIntervalInvalid = errors.New("intrusive interval is invalid")
