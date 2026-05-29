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

		now := s.clock.Now()

		reminders, err := s.database.ActualReminders(ctx, now, now.Add(s.pollInterval))
		if err != nil {
			return fmt.Errorf("load scheduled reminders: %w", err)
		}

		s.enqueue(reminders)

		if err = s.sleep(ctx, s.pollInterval); err != nil {
			return err
		}
	}
}

func (s *Scheduler) enqueue(reminders []*database.ScheduledReminder) {
	if len(reminders) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.queue = append(s.queue, reminders...)
	s.queueReady.Broadcast()
}

func (s *Scheduler) sleep(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait polling interval: %w", ctx.Err())
	case <-s.clock.After(delay):
		return nil
	}
}
