package apperrors

import (
	"errors"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
)

func mapErrorResponse(err error) ErrorResponse {
	switch {
	case errors.Is(err, apphttp.ErrTelegramChatIDMissing),
		errors.Is(err, apphttp.ErrTelegramChatIDInvalid),
		errors.Is(err, apphttp.ErrInvalidRequest),
		errors.Is(err, apphttp.ErrInvalidID),
		errors.Is(err, apphttp.ErrInvalidItemID),
		errors.Is(err, apphttp.ErrInvalidOffset),
		errors.Is(err, apphttp.ErrInvalidLimit),
		errors.Is(err, apphttp.ErrReminderStatusesEmpty),
		errors.Is(err, apphttp.ErrReminderStatusInvalid),
		errors.Is(err, apphttp.ErrQueryContentTypeMissing),
		errors.Is(err, apphttp.ErrReminderRepeatIntervalDaysEmpty),
		errors.Is(err, apphttp.ErrReminderRepeatIntervalDaysInvalid),
		errors.Is(err, apphttp.ErrReminderRepeatWeekdaysMaskEmpty),
		errors.Is(err, apphttp.ErrReminderRepeatCustomDatesEmpty),
		errors.Is(err, apphttp.ErrReminderRepeatCustomTimeEmpty),
		errors.Is(err, apphttp.ErrReminderRepeatCustomTimeInvalid),
		errors.Is(err, apphttp.ErrReminderRepeatTypeUnsupported),
		errors.Is(err, apphttp.ErrReminderRepeatDateInvalid),
		errors.Is(err, apphttp.ErrReminderRepeatTimezoneInvalid),
		errors.Is(err, apphttp.ErrHolidayNameEmpty),
		errors.Is(err, apphttp.ErrHolidayRecipientNameEmpty),
		errors.Is(err, apphttp.ErrHolidayDateInvalid),
		errors.Is(err, apphttp.ErrHolidayRemindAtEmpty),
		errors.Is(err, apphttp.ErrShoppingListItemsEmpty),
		errors.Is(err, apphttp.ErrShoppingListItemTextEmpty),
		errors.Is(err, apphttp.ErrSettingsTimezoneInvalid),
		errors.Is(err, apphttp.ErrIntrusiveIntervalInvalid):
		return newErrorResponse(http.StatusBadRequest, err.Error())

	case errors.Is(err, service.ErrReminderTitleEmpty),
		errors.Is(err, service.ErrWeekdaysMaskEmpty),
		errors.Is(err, service.ErrReminderStatusUnsupported):
		return newErrorResponse(http.StatusBadRequest, err.Error())

	case errors.Is(err, service.ErrReminderNotFound):
		return newErrorResponse(http.StatusNotFound, service.ErrReminderNotFound.Error())

	case errors.Is(err, service.ErrHolidayNotFound):
		return newErrorResponse(http.StatusNotFound, service.ErrHolidayNotFound.Error())

	case errors.Is(err, service.ErrShoppingListNotFound):
		return newErrorResponse(http.StatusNotFound, service.ErrShoppingListNotFound.Error())

	case errors.Is(err, service.ErrAccessDenied):
		return newErrorResponse(http.StatusForbidden, service.ErrAccessDenied.Error())

	case errors.Is(err, apphttp.ErrQueryContentTypeUnsupported):
		return newErrorResponse(http.StatusUnsupportedMediaType, apphttp.ErrQueryContentTypeUnsupported.Error())

	default:
		return newErrorResponse(http.StatusInternalServerError, "internal server error")
	}
}
