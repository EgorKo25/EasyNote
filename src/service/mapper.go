package service

import (
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/domain"
)

func mapReminderViewsToDomain(reminders []database.ReminderView) []domain.ReminderView {
	result := make([]domain.ReminderView, 0, len(reminders))
	for _, reminder := range reminders {
		result = append(result, mapReminderViewToDomain(reminder))
	}

	return result
}

func mapReminderViewToDomain(reminder database.ReminderView) domain.ReminderView {
	return domain.ReminderView{
		ID:          reminder.ID,
		RuleID:      reminder.RuleID,
		Title:       reminder.Title,
		Description: reminder.Description,
		RemindAt:    reminder.RemindAt,
		Status:      domain.ReminderStatus(reminder.Status),
		CreatedAt:   reminder.CreatedAt,
	}
}

func mapReminderStatsToDomain(stats database.ReminderStats) domain.ReminderStats {
	return domain.ReminderStats{
		Completed: stats.Completed,
		Pending:   stats.Pending,
		Skipped:   stats.Skipped,
	}
}

func mapHolidayViewsToDomain(holidays []database.HolidayView) []domain.HolidayView {
	result := make([]domain.HolidayView, 0, len(holidays))
	for _, holiday := range holidays {
		result = append(result, mapHolidayViewToDomain(holiday))
	}

	return result
}

func mapHolidayViewToDomain(holiday database.HolidayView) domain.HolidayView {
	return domain.HolidayView{
		ID:             holiday.ID,
		TelegramChatID: holiday.TelegramChatID,
		HolidayName:    holiday.HolidayName,
		RecipientName:  holiday.RecipientName,
		HolidayDate:    holiday.HolidayDate,
		RemindAt:       holiday.RemindAt,
		Status:         domain.HolidayStatus(holiday.Status),
		CreatedAt:      holiday.CreatedAt,
	}
}

func mapHolidayRequestToDatabase(request domain.HolidayRequest) database.HolidayRequest {
	return database.HolidayRequest{
		TelegramChatID: request.TelegramChatID,
		HolidayName:    request.HolidayName,
		RecipientName:  request.RecipientName,
		HolidayDate:    request.HolidayDate,
		RemindAt:       request.RemindAt,
	}
}

func mapHolidayUpdateRequestToDatabase(request domain.HolidayUpdateRequest) database.HolidayUpdateRequest {
	return database.HolidayUpdateRequest{
		ID:             request.ID,
		HolidayRequest: mapHolidayRequestToDatabase(request.HolidayRequest),
	}
}

func mapShoppingListViewsToDomain(lists []database.ShoppingListView) []domain.ShoppingListView {
	result := make([]domain.ShoppingListView, 0, len(lists))
	for _, list := range lists {
		result = append(result, mapShoppingListViewToDomain(list))
	}

	return result
}

func mapShoppingListViewToDomain(list database.ShoppingListView) domain.ShoppingListView {
	return domain.ShoppingListView{
		ID:             list.ID,
		TelegramChatID: list.TelegramChatID,
		CreatedAt:      list.CreatedAt,
		Items:          mapShoppingListItemsToDomain(list.Items),
	}
}

func mapShoppingListItemsToDomain(items []database.ShoppingListItem) []domain.ShoppingListItem {
	result := make([]domain.ShoppingListItem, 0, len(items))
	for _, item := range items {
		result = append(result, domain.ShoppingListItem{
			ID:        item.ID,
			ListID:    item.ListID,
			Text:      item.Text,
			Done:      item.Done,
			Position:  item.Position,
			CreatedAt: item.CreatedAt,
		})
	}

	return result
}

func mapShoppingListItemsToDatabase(items []domain.ShoppingListItem) []database.ShoppingListItem {
	result := make([]database.ShoppingListItem, 0, len(items))
	for _, item := range items {
		result = append(result, database.ShoppingListItem{
			ID:        item.ID,
			ListID:    item.ListID,
			Text:      item.Text,
			Done:      item.Done,
			Position:  item.Position,
			CreatedAt: item.CreatedAt,
		})
	}

	return result
}

func mapUserSettingsToDomain(settings database.UserSettings) domain.UserSettings {
	return domain.UserSettings{
		TelegramChatID:    settings.TelegramChatID,
		Language:          settings.Language,
		Timezone:          settings.Timezone,
		IntrusiveEnabled:  settings.IntrusiveEnabled,
		IntrusiveInterval: settings.IntrusiveInterval,
		UpdatedAt:         settings.UpdatedAt,
	}
}
