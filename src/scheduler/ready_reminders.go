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

		status := database.ReminderStatusSent

		var sentAt *time.Time

		sendErr := r.retrier.Retry(
			ctx,
			func() error {
				return r.sender.SendReminder(ctx, *reminder)
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
		if sendErr != nil {
			if errors.Is(sendErr, context.Canceled) || errors.Is(sendErr, context.DeadlineExceeded) {
				return fmt.Errorf("send reminder %d: %w", reminder.ID, sendErr)
			}

			status = database.ReminderStatusFailed

			logger.Get("scheduler/runner").Error(
				"cannot send reminder",
				sendErr,
				logger.Int64("reminder_id", reminder.ID),
			)
		} else {
			sentTime := r.clock.Now()
			sentAt = &sentTime
		}

		if err = r.planner.CompleteOccurrence(ctx, reminder.ID, status, sentAt); err != nil {
			return fmt.Errorf("complete reminder %d with status %q: %w", reminder.ID, status, err)
		}
	}

	return nil
}
