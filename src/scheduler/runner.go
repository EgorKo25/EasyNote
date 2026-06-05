package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
)

var errRetryCountExhausted = errors.New("retry count is exhausted")

func NewRunner(
	storage Storage,
	sender Sender,
	planner Planner,
	cfg config.Scheduler,
	opts ...Option,
) *Runner {
	runner := &Runner{
		storage: storage,
		sender:  sender,
		planner: planner,
		cfg:     cfg,
		clock:   newSystemClock(),
		log:     logger.Get("scheduler/runner"),
	}

	for _, opt := range opts {
		opt(runner)
	}

	return runner
}

type Runner struct {
	storage Storage
	sender  Sender
	planner Planner
	cfg     config.Scheduler
	clock   Clock
	log     logger.Logger
}

//go:generate go run go.uber.org/mock/mockgen -source=runner.go -destination=mocks/scheduler_mock.go -package=mocks
type Storage interface {
	FindRemindersReadyToSend(
		ctx context.Context,
		now time.Time,
		limit int,
	) ([]*database.ScheduledReminder, error)
}

type Sender interface {
	SendReminder(ctx context.Context, reminder database.ScheduledReminder) error
}

type Planner interface {
	CompleteOccurrence(
		ctx context.Context,
		reminderID int64,
		status database.ReminderStatus,
		sentAt *time.Time,
	) error
}

func (r *Runner) Info() string {
	return fmt.Sprintf("run scheduler reminder runner, with poll interval: %s", r.cfg.PollInterval)
}

func (r *Runner) Run(ctx context.Context) error {
	for {
		if err := r.waitPollInterval(ctx); err != nil {
			return err
		}

		if err := r.processReadyReminders(ctx); err != nil {
			return err
		}
	}
}

func (r *Runner) processReadyReminders(ctx context.Context) error {
	reminders, err := r.storage.FindRemindersReadyToSend(ctx, r.clock.Now(), r.cfg.BatchLimit)
	if err != nil {
		return fmt.Errorf("find reminders ready to send: %w", err)
	}

	for _, reminder := range reminders {
		if err = ctx.Err(); err != nil {
			return fmt.Errorf("process ready reminders: %w", err)
		}

		if err = r.processReminder(ctx, *reminder); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) processReminder(ctx context.Context, reminder database.ScheduledReminder) error {
	sentAt, err := r.sendWithRetry(ctx, reminder)
	if err != nil {
		if markErr := r.planner.CompleteOccurrence(
			ctx,
			reminder.ID,
			database.ReminderStatusFailed,
			nil,
		); markErr != nil {
			return fmt.Errorf(
				"complete reminder %d as failed after send error %q: %w",
				reminder.ID,
				err.Error(),
				markErr,
			)
		}

		r.log.Error("cannot send reminder", err, logger.Int64("reminder_id", reminder.ID))

		return nil
	}

	if err = r.planner.CompleteOccurrence(ctx, reminder.ID, database.ReminderStatusSent, &sentAt); err != nil {
		return fmt.Errorf("complete reminder %d as sent: %w", reminder.ID, err)
	}

	return nil
}

func (r *Runner) sendWithRetry(ctx context.Context, reminder database.ScheduledReminder) (time.Time, error) {
	var err error

	for attempt := 1; attempt <= r.cfg.SendRetryCount; attempt++ {
		if err = ctx.Err(); err != nil {
			return time.Time{}, fmt.Errorf("send reminder %d: %w", reminder.ID, err)
		}

		if err = r.sender.SendReminder(ctx, reminder); err == nil {
			return r.clock.Now(), nil
		}

		if attempt == r.cfg.SendRetryCount {
			return time.Time{}, fmt.Errorf("send reminder %d after %d attempts: %w", reminder.ID, attempt, err)
		}

		r.log.Warn(
			"cannot send reminder, retrying",
			logger.Any("error", err),
			logger.Int64("reminder_id", reminder.ID),
			logger.Int("attempt", attempt),
		)

		if waitErr := r.waitRetryInterval(ctx); waitErr != nil {
			return time.Time{}, waitErr
		}
	}

	return time.Time{}, fmt.Errorf("send reminder %d: %w", reminder.ID, errRetryCountExhausted)
}

func (r *Runner) waitPollInterval(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("run scheduler reminder runner: %w", ctx.Err())
	case <-r.clock.After(r.cfg.PollInterval):
		return nil
	}
}

func (r *Runner) waitRetryInterval(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait send retry interval: %w", ctx.Err())
	case <-r.clock.After(r.cfg.SendRetryInterval):
		return nil
	}
}
