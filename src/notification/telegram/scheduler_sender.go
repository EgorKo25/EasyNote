package telegram

import (
	"context"

	"github.com/EgorKo25/EasyNote/src/database"
)

type SchedulerSender struct {
	sender *Sender
}

func NewSchedulerSender(sender *Sender) *SchedulerSender {
	return &SchedulerSender{sender: sender}
}

func (s *SchedulerSender) SendReminder(ctx context.Context, reminder database.ScheduledReminder) (int, error) {
	return s.sender.SendReminder(ctx, Reminder{
		ID:                reminder.ID,
		NoteID:            reminder.NoteID,
		RuleID:            reminder.RuleID,
		TelegramChatID:    reminder.TelegramChatID,
		Title:             reminder.Title,
		Description:       reminder.Description,
		RemindAt:          reminder.RemindAt,
		Index:             reminder.Index,
		IntrusiveEnabled:  reminder.IntrusiveEnabled,
		IntrusiveInterval: reminder.IntrusiveInterval,
		Timezone:          reminder.Timezone,
		TelegramMessageID: reminder.TelegramMessageID,
	})
}

func (s *SchedulerSender) SendHoliday(ctx context.Context, holiday database.ScheduledHoliday) (int, error) {
	return s.sender.SendHoliday(ctx, Holiday{
		ID:             holiday.ID,
		TelegramChatID: holiday.TelegramChatID,
		HolidayName:    holiday.HolidayName,
		RecipientName:  holiday.RecipientName,
		RemindAt:       holiday.RemindAt,
		Timezone:       holiday.Timezone,
	})
}

func (s *SchedulerSender) DeleteReminderMessage(ctx context.Context, reminder database.ScheduledReminder) error {
	return s.sender.DeleteReminderMessage(ctx, Reminder{
		ID:                reminder.ID,
		NoteID:            reminder.NoteID,
		RuleID:            reminder.RuleID,
		TelegramChatID:    reminder.TelegramChatID,
		Title:             reminder.Title,
		Description:       reminder.Description,
		RemindAt:          reminder.RemindAt,
		Index:             reminder.Index,
		IntrusiveEnabled:  reminder.IntrusiveEnabled,
		IntrusiveInterval: reminder.IntrusiveInterval,
		Timezone:          reminder.Timezone,
		TelegramMessageID: reminder.TelegramMessageID,
	})
}
