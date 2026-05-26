package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

var errReminderProcessorRequired = errors.New("reminder processor is required")

func (s *Scheduler) Start(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check scheduler context: %w", err)
		}

		reminders, err := s.loadPollingWindow(ctx)
		if err != nil {
			return err
		}

		if len(reminders) > 0 {
			if s.processor == nil {
				return errReminderProcessorRequired
			}

			if err := s.processor.ProcessReminders(ctx, reminders); err != nil {
				return fmt.Errorf("process due reminders: %w", err)
			}
		}

		if err := s.sleep(ctx, s.pollInterval); err != nil {
			return err
		}
	}
}

func (s *Scheduler) loadPollingWindow(ctx context.Context) ([]database.ScheduledReminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()

	reminders, err := s.database.ScheduledReminders(ctx, now, now.Add(s.pollInterval))
	if err != nil {
		return nil, fmt.Errorf("load scheduled reminders: %w", err)
	}

	return reminders, nil
}

func (s *Scheduler) sleep(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait polling interval: %w", ctx.Err())
	case <-s.clock.After(delay):
		return nil
	}
}
