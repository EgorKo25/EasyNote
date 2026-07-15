package commands

import (
	"github.com/EgorKo25/EasyNote/src/database"
)

func mapRemindersViewToResponse(reminders []database.ReminderView) []ReminderDTO {
	remindersDTO := make([]ReminderDTO, 0, len(reminders))

	for _, rem := range reminders {
		remindersDTO = append(remindersDTO, mapReminderViewToDTO(rem))
	}

	return remindersDTO
}

func mapReminderViewToResponse(reminder database.ReminderView) CreateReminderResponse {
	return CreateReminderResponse{
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
