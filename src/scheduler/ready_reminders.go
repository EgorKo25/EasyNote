package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
)

func (r *Runner) sendDueReminders(ctx context.Context) error {
	now := r.Now()

	reminders, err := r.FindRemindersReadyToSend(ctx, now, r.cfg.BatchLimit)
	if err != nil {
		return fmt.Errorf("find reminders ready to send: %w", err)
	}

	for _, reminder := range reminders {
		if err = ctx.Err(); err != nil {
			return fmt.Errorf("send due reminders: %w", err)
		}

		if err = r.sendDueReminder(ctx, *reminder, now); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) sendDueReminder(
	ctx context.Context,
	reminder database.ScheduledReminder,
	now time.Time,
) error {
	if shouldSkipIntrusiveReminder(reminder, now) {
		if err := r.DeleteReminderMessage(ctx, reminder); err != nil {
			return fmt.Errorf("delete expired intrusive reminder %d message: %w", reminder.ID, err)
		}

		return r.CompleteOccurrence(ctx, reminder.ID, database.ReminderStatusSkipped, &now)
	}

	status := database.ReminderStatusSent

	messageID, err := r.sendReminder(ctx, reminder)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("send reminder %d: %w", reminder.ID, err)
		}

		status = database.ReminderStatusFailed

		logger.Get("scheduler/runner").Error(
			"cannot send reminder",
			err,
			logger.Int64("reminder_id", reminder.ID),
		)
	}

	var completedAt *time.Time
	if status == database.ReminderStatusSent {
		completedAt = &now
	}

	if reminder.IntrusiveEnabled {
		if err = r.MarkReminderDelivered(ctx,
			deliveryForReminder(reminder, messageID, now)); err != nil {
			return fmt.Errorf("mark reminder %d delivered: %w", reminder.ID, err)
		}

		return nil
	}

	if err = r.CompleteOccurrence(ctx, reminder.ID, status, completedAt); err != nil {
		return fmt.Errorf("complete reminder %d with status %q: %w", reminder.ID, status, err)
	}

	return nil
}

func (r *Runner) sendReminder(
	ctx context.Context,
	reminder database.ScheduledReminder,
) (int, error) {
	var messageID int

	err := r.Retry(
		ctx,
		func() error {
			sentMessageID, err := r.SendReminder(ctx, reminder)
			if err != nil {
				return fmt.Errorf("send reminder: %w", err)
			}

			messageID = sentMessageID

			return nil
		},
		func(err error, delay time.Duration) {
			logger.Get("scheduler/runner").Warn(
				"cannot send reminder, retrying",
				logger.Any("error", err),
				logger.Int64("reminder_id", reminder.ID),
				logger.String("retry_after", delay.String()),
			)
		},
	)
	if err != nil {
		return 0, fmt.Errorf("retry reminder send: %w", err)
	}

	return messageID, nil
}

func shouldSkipIntrusiveReminder(reminder database.ScheduledReminder, now time.Time) bool {
	return reminder.IntrusiveEnabled &&
		!intrusiveReminderCutoff(reminder).After(now)
}

func intrusiveReminderCutoff(reminder database.ScheduledReminder) time.Time {
	location, err := time.LoadLocation(reminder.Timezone)
	if err != nil {
		location = time.Local
	}

	localRemindAt := reminder.RemindAt.In(location)

	cutoff := time.Date(
		localRemindAt.Year(),
		localRemindAt.Month(),
		localRemindAt.Day(),
		12,
		0,
		0,
		0,
		location,
	)
	if !cutoff.After(localRemindAt) {
		cutoff = cutoff.AddDate(0, 0, 1)
	}

	return cutoff
}

func deliveryForReminder(
	reminder database.ScheduledReminder,
	messageID int,
	sentAt time.Time,
) database.ReminderDelivery {
	interval := reminder.IntrusiveInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}

	intrusiveUntil := intrusiveReminderCutoff(reminder)
	if messageID == 0 && reminder.TelegramMessageID != nil {
		messageID = *reminder.TelegramMessageID
	}

	return database.ReminderDelivery{
		ReminderID:     reminder.ID,
		MessageID:      messageID,
		NextSendAt:     sentAt.Add(interval),
		IntrusiveUntil: intrusiveUntil,
	}
}
