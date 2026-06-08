package scheduler

import (
	"context"
	"fmt"
	"time"
)

func (r *Runner) Run(ctx context.Context) error {
	for {
		if err := wait(ctx, r.Clock, r.cfg.PollInterval, "wait scheduler interval"); err != nil {
			return err
		}

		if err := r.sendDueReminders(ctx); err != nil {
			return err
		}
	}
}

func wait(ctx context.Context, clock Clock, delay time.Duration, operation string) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", operation, ctx.Err())
	case <-clock.After(delay):
		return nil
	}
}
