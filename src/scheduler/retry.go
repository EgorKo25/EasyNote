package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"

	"github.com/EgorKo25/EasyNote/src/config"
)

const RetryBackoffExponential = "exponential"

var errRetryAttemptsExhausted = errors.New("retry attempts exhausted")

func NewBackoffRetrier(cfg config.Scheduler, clock Clock) *BackoffRetrier {
	return &BackoffRetrier{
		attempts:    cfg.SendRetryCount,
		interval:    cfg.SendRetryInterval,
		maxInterval: cfg.SendRetryMaxInterval,
		strategy:    cfg.SendRetryBackoff,
		clock:       clock,
	}
}

type BackoffRetrier struct {
	attempts    int
	interval    time.Duration
	maxInterval time.Duration
	strategy    string
	clock       Clock
}

func (r *BackoffRetrier) Retry(
	ctx context.Context,
	operation func() error,
	notify func(error, time.Duration),
) error {
	attempts := r.attempts
	if attempts == 0 {
		attempts = 1
	}

	policy := r.newBackOff()
	policy.Reset()

	var lastErr error

	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check retry context: %w", err)
		}

		if err := operation(); err != nil {
			lastErr = err
		} else {
			return nil
		}

		if attempt >= attempts {
			return fmt.Errorf("%w: %w", errRetryAttemptsExhausted, lastErr)
		}

		delay := policy.NextBackOff()
		if delay == backoff.Stop {
			return fmt.Errorf("%w: %w", errRetryAttemptsExhausted, lastErr)
		}

		if notify != nil {
			notify(lastErr, delay)
		}

		if err := wait(ctx, r.clock, delay, "wait retry backoff"); err != nil {
			return err
		}
	}
}

func (r *BackoffRetrier) newBackOff() backoff.BackOff {
	if r.strategy == RetryBackoffExponential {
		policy := backoff.NewExponentialBackOff()
		policy.InitialInterval = r.interval
		policy.MaxInterval = r.maxInterval
		policy.Multiplier = 2
		policy.RandomizationFactor = 0

		return policy
	}

	return backoff.NewConstantBackOff(r.interval)
}
