package commands

import "errors"

var ErrInvalidRequest = errors.New("invalid request")
var ErrInvalidOffset = errors.New("invalid offset")
var ErrInvalidLimit = errors.New("invalid limit")

var ErrReminderRepeatIntervalDaysEmpty = errors.New("reminder repeat interval_days is empty")
var ErrReminderRepeatWeekdaysMaskEmpty = errors.New("reminder repeat weekdays_mask is empty")
var ErrReminderRepeatCustomTimeEmpty = errors.New("reminder repeat custom time is empty")
var ErrReminderRepeatTypeUnsupported = errors.New("unsupported reminder repeat type")
var ErrReminderRepeatDateInvalid = errors.New("reminder repeat date is invalid")
var ErrReminderRepeatTimezoneInvalid = errors.New("reminder repeat timezone is invalid")
