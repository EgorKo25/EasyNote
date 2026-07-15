package database

import "errors"

var ErrReminderNotFound = errors.New("reminder not found")
var ErrHolidayNotFound = errors.New("holiday not found")
var ErrShoppingListNotFound = errors.New("shopping list not found")
