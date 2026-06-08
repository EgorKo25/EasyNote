package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
)

func (r *Runner) processReadyReminders(ctx context.Context) error {
	reminders, err := r.storage.FindRemindersReadyToSend(ctx, r.clock.Now(), r.cfg.BatchLimit)
	if err != nil {
		return fmt.Errorf("find reminders ready to send: %w", err)
	}

	for _, reminder := range reminders {
		if err = ctx.Err(); err != nil {
			return fmt.Errorf("process ready reminders: %w", err)
		}

		if err = r.processReadyReminder(ctx, *reminder); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) processReadyReminder(ctx context.Context, reminder database.ScheduledReminder) error {
	sentTime := r.clock.Now()
	if shouldSkipIntrusiveReminder(reminder, sentTime) {
		return r.planner.CompleteOccurrence(ctx, reminder.ID, database.ReminderStatusSkipped, &sentTime)
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

	sendTime := r.clock.Now()

	if reminder.IntrusiveEnabled {
		if err = r.storage.MarkReminderDelivered(ctx,
			deliveryForReminder(reminder, messageID, sendTime)); err != nil {
			return fmt.Errorf("mark reminder %d delivered: %w", reminder.ID, err)
		}

		return nil
	}

	return r.planner.CompleteOccurrence(ctx, reminder.ID, status, sendTime)
}

func (r *Runner) sendReminder(
	ctx context.Context,
	reminder database.ScheduledReminder,
) (int, error) {
	var messageID int

	err := r.retrier.Retry(
		ctx,
		func() error {
			sentMessageID, err := r.sender.SendReminder(ctx, reminder)
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
		return 0, nil, fmt.Errorf("retry reminder send: %w", err)
	}

	return messageID, nil
}

func (r *Runner) (
	ctx context.Context,
	reminderID int64,
	status database.ReminderStatus,
	sentAt *time.Time,
) error {
	if err := r.planner.CompleteOccurrence(ctx, reminderID, status, sentAt); err != nil {
		return fmt.Errorf("complete reminder %d with status %q: %w", reminderID, status, err)
	}

	return nil
}

func shouldSkipIntrusiveReminder(reminder database.ScheduledReminder, now time.Time) bool {
	return reminder.IntrusiveEnabled &&
		!reminder.RemindAt.Add(24*time.Hour).After(now)
}

func deliveryForReminder(
	reminder database.ScheduledReminder,
	messageID int,
	sentAt time.Time,
) database.ReminderDelivery {
	interval := reminder.IntrusiveInterval
	interval = max(interval, 30*time.Minute)

	intrusiveUntil := reminder.RemindAt.Add(24 * time.Hour)
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
