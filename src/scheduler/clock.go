package scheduler

import "github.com/benbjohnson/clock"

type Clock = clock.Clock

type Option func(*Runner)

func WithClock(clock Clock) Option {
	return func(r *Runner) {
		r.Clock = clock
	}
}

func WithRetrier(retrier Retrier) Option {
	return func(r *Runner) {
		r.Retrier = retrier
	}
}

func newSystemClock() Clock {
	return clock.New()
}
