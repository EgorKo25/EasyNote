package commands

import (
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
)

func mapRemindersViewToResponse(reminders []database.ReminderView) []ReminderDTO {
	remindersDTO := make([]ReminderDTO, 0, len(reminders))

	for _, rem := range reminders {
		remindersDTO = append(remindersDTO, mapReminderViewToDTO(rem))
	}

	return remindersDTO
}

func mapReminderViewToResponse(reminder database.ReminderView) ReminderResponse {
	return ReminderResponse{
		Reminder: mapReminderViewToDTO(reminder),
	}
}

func mapReminderViewToDTO(reminder database.ReminderView) ReminderDTO {
	return ReminderDTO{
		ID:          reminder.ID,
		RuleID:      reminder.RuleID,
		Title:       reminder.Title,
		Description: reminder.Description,
		RemindAt:    reminder.RemindAt,
		Status:      reminder.Status,
		CreatedAt:   reminder.CreatedAt,
	}
}

func mapHolidayViewToResponse(holiday database.HolidayView) HolidayResponse {
	return HolidayResponse{
		Holiday: mapHolidayViewToDTO(holiday),
	}
}

func mapHolidaysViewToResponse(holidays []database.HolidayView) []HolidayDTO {
	holidaysDTO := make([]HolidayDTO, 0, len(holidays))

	for _, holiday := range holidays {
		holidaysDTO = append(holidaysDTO, mapHolidayViewToDTO(holiday))
	}

	return holidaysDTO
}

func mapHolidayViewToDTO(holiday database.HolidayView) HolidayDTO {
	return HolidayDTO{
		ID:             holiday.ID,
		TelegramChatID: holiday.TelegramChatID,
		HolidayName:    holiday.HolidayName,
		RecipientName:  holiday.RecipientName,
		HolidayDate:    holiday.HolidayDate,
		RemindAt:       holiday.RemindAt,
		Status:         holiday.Status,
		CreatedAt:      holiday.CreatedAt,
	}
}

func mapCreateHolidayRequestToDatabaseRequest(request CreateHolidayRequest) database.HolidayRequest {
	return database.HolidayRequest{
		TelegramChatID: request.TelegramChatID,
		HolidayName:    request.HolidayName,
		RecipientName:  request.RecipientName,
		HolidayDate:    request.HolidayDateValue,
		RemindAt:       request.RemindAt,
	}
}

func mapUpdateHolidayRequestToDatabaseRequest(request UpdateHolidayRequest) database.HolidayUpdateRequest {
	return database.HolidayUpdateRequest{
		ID: request.HolidayID,
		HolidayRequest: database.HolidayRequest{
			TelegramChatID: request.TelegramChatID,
			HolidayName:    request.HolidayName,
			RecipientName:  request.RecipientName,
			HolidayDate:    request.HolidayDateValue,
			RemindAt:       request.RemindAt,
		},
	}
}

func mapShoppingListViewToResponse(list database.ShoppingListView) ShoppingListResponse {
	return ShoppingListResponse{
		ShoppingList: mapShoppingListViewToDTO(list),
	}
}

func mapShoppingListsViewToResponse(lists []database.ShoppingListView) []ShoppingListDTO {
	listsDTO := make([]ShoppingListDTO, 0, len(lists))

	for _, list := range lists {
		listsDTO = append(listsDTO, mapShoppingListViewToDTO(list))
	}

	return listsDTO
}

func mapShoppingListViewToDTO(list database.ShoppingListView) ShoppingListDTO {
	return ShoppingListDTO{
		ID:             list.ID,
		TelegramChatID: list.TelegramChatID,
		CreatedAt:      list.CreatedAt,
		Items:          mapShoppingListItemsToDTO(list.Items),
	}
}

func mapShoppingListItemsToDTO(items []database.ShoppingListItem) []ShoppingListItemDTO {
	itemsDTO := make([]ShoppingListItemDTO, 0, len(items))

	for _, item := range items {
		itemsDTO = append(itemsDTO, mapShoppingListItemToDTO(item))
	}

	return itemsDTO
}

func mapShoppingListItemToDTO(item database.ShoppingListItem) ShoppingListItemDTO {
	return ShoppingListItemDTO{
		ID:        item.ID,
		ListID:    item.ListID,
		Text:      item.Text,
		Done:      item.Done,
		Position:  item.Position,
		CreatedAt: item.CreatedAt,
	}
}

func mapCreateShoppingListItemsToDatabaseItems(items []CreateShoppingListItemRequest) []database.ShoppingListItem {
	databaseItems := make([]database.ShoppingListItem, 0, len(items))

	for index, item := range items {
		databaseItems = append(databaseItems, database.ShoppingListItem{
			Text:     item.Text,
			Position: index + 1,
		})
	}

	return databaseItems
}

func mapReminderStatsToDTO(stats database.ReminderStats) ReminderStatsDTO {
	return ReminderStatsDTO{
		Completed: stats.Completed,
		Pending:   stats.Pending,
		Skipped:   stats.Skipped,
	}
}

func mapUserSettingsToDTO(settings database.UserSettings) UserSettingsDTO {
	return UserSettingsDTO{
		TelegramChatID:           settings.TelegramChatID,
		Language:                 settings.Language,
		Timezone:                 settings.Timezone,
		IntrusiveEnabled:         settings.IntrusiveEnabled,
		IntrusiveIntervalMinutes: int(settings.IntrusiveInterval / time.Minute),
		UpdatedAt:                settings.UpdatedAt,
	}
}

func mapTimezoneOptionsToResponse(options []service.TimezoneOption) []TimezoneDTO {
	timezones := make([]TimezoneDTO, 0, len(options))

	for _, option := range options {
		timezones = append(timezones, TimezoneDTO{
			City:     option.City,
			Timezone: option.Timezone,
		})
	}

	return timezones
}
