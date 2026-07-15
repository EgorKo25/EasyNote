package service

import "errors"

var ErrReminderTitleEmpty = errors.New("reminder title is empty")
var ErrWeekdaysMaskEmpty = errors.New("weekdays mask is empty")
var ErrReminderNotFound = errors.New("reminder not found")
var ErrHolidayNotFound = errors.New("holiday not found")
var ErrShoppingListNotFound = errors.New("shopping list not found")
var ErrAccessDenied = errors.New("access denied")
