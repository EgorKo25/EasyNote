package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

func (s *Scheduler) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check scheduler context: %w", err)
		}

		reminders, err := s.loadPollingWindow(ctx)
		if err != nil {
			return err
		}

		if err = s.processor.ProcessReminders(ctx, reminders); err != nil {
			return fmt.Errorf("process due reminders: %w", err)
		}

		if err = s.sleep(ctx, s.pollInterval); err != nil {
			return err
		}
	}
}

func (s *Scheduler) loadPollingWindow(ctx context.Context) ([]*database.ScheduledReminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()

	reminders, err := s.database.ActualReminders(ctx, now, now.Add(s.pollInterval))
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
