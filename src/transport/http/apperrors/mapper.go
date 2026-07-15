package apperrors

import (
	"errors"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/transport/http/commands"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
)

func mapErrorResponse(err error) ErrorResponse {
	switch {
	case errors.Is(err, middleware.ErrTelegramChatIDMissing),
		errors.Is(err, middleware.ErrTelegramChatIDInvalid),
		errors.Is(err, commands.ErrInvalidRequest),
		errors.Is(err, commands.ErrInvalidOffset),
		errors.Is(err, commands.ErrInvalidLimit),
		errors.Is(err, commands.ErrReminderRepeatIntervalDaysEmpty),
		errors.Is(err, commands.ErrReminderRepeatWeekdaysMaskEmpty),
		errors.Is(err, commands.ErrReminderRepeatCustomTimeEmpty),
		errors.Is(err, commands.ErrReminderRepeatTypeUnsupported),
		errors.Is(err, commands.ErrReminderRepeatDateInvalid),
		errors.Is(err, commands.ErrReminderRepeatTimezoneInvalid),
		errors.Is(err, service.ErrReminderTitleEmpty),
		errors.Is(err, service.ErrWeekdaysMaskEmpty):
		return newErrorResponse(http.StatusBadRequest, err.Error())

	case errors.Is(err, service.ErrReminderNotFound),
		errors.Is(err, service.ErrHolidayNotFound),
		errors.Is(err, service.ErrShoppingListNotFound):
		return newErrorResponse(http.StatusNotFound, err.Error())

	case errors.Is(err, service.ErrAccessDenied):
		return newErrorResponse(http.StatusForbidden, err.Error())

	case errors.Is(err, database.ErrHolidayNotFound),
		errors.Is(err, database.ErrReminderNotFound),
		errors.Is(err, database.ErrShoppingListNotFound):
		return newErrorResponse(http.StatusNotFound, err.Error())

	default:
		return newErrorResponse(http.StatusInternalServerError, "internal server error")
	}
}
