package service

import (
	"context"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
)

func NewReminderWorker(repository ReminderRepository, sender ReminderSender, cfg config.Scheduler) *ReminderWorker {
	return &ReminderWorker{
		repository: repository,
		sender:     sender,
		cfg:        cfg,
		now:        time.Now,
	}
}

type ReminderWorker struct {
	repository ReminderRepository
	sender     ReminderSender
	cfg        config.Scheduler
	now        func() time.Time
}

//go:generate go run go.uber.org/mock/mockgen -source=reminder_worker.go -destination=mocks/reminder_worker_mock.go -package=mocks
type ReminderRepository interface {
	FindRemindersReadyToSend(
		ctx context.Context,
		now time.Time,
		limit, maxAttempts int,
	) ([]*database.ScheduledReminder, error)
	CompleteReminder(ctx context.Context, request database.ReminderCompletionRequest) error
}

type ReminderSender interface {
	SendReminder(ctx context.Context, reminder database.ScheduledReminder) error
}

func (w *ReminderWorker) Info() string {
	return fmt.Sprintf("run reminder worker, with poll interval: %s", w.cfg.PollInterval)
}

func (w *ReminderWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("run reminder worker: %w", ctx.Err())
		case <-ticker.C:
			if err := w.sendReadyReminders(ctx); err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return fmt.Errorf("run reminder worker: %w", err)
			}
		}
	}
}

func (w *ReminderWorker) sendReadyReminders(ctx context.Context) error {
	reminders, err := w.repository.FindRemindersReadyToSend(ctx, w.now(), w.cfg.BatchLimit, w.cfg.MaxAttempts)
	if err != nil {
		return fmt.Errorf("find reminders ready to send: %w", err)
	}

	if len(reminders) == 0 {
		return nil
	}

	log := logger.Get("service/reminder-worker")

	for _, reminder := range reminders {
		if err = ctx.Err(); err != nil {
			return fmt.Errorf("send ready reminders: %w", err)
		}

		if err = w.sender.SendReminder(ctx, *reminder); err != nil {
			if markErr := w.repository.CompleteReminder(ctx, database.ReminderCompletionRequest{
				ReminderID:  reminder.ID,
				Status:      database.ReminderCompletionStatusFailed,
				MaxAttempts: w.cfg.MaxAttempts,
				Next:        CalculateNextReminder,
			}); markErr != nil {
				return fmt.Errorf("mark reminder %d failed after send error %q: %w", reminder.ID, err.Error(), markErr)
			}

			log.Error("cannot send reminder", err, logger.Int64("reminder_id", reminder.ID))

			continue
		}

		sentAt := w.now()
		if err = w.repository.CompleteReminder(ctx, database.ReminderCompletionRequest{
			ReminderID: reminder.ID,
			Status:     database.ReminderCompletionStatusSent,
			SentAt:     &sentAt,
			Next:       CalculateNextReminder,
		}); err != nil {
			return fmt.Errorf("mark reminder %d sent: %w", reminder.ID, err)
		}
	}

	return nil
}
