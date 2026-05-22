package scheduler

import "github.com/benbjohnson/clock"

type Clock = clock.Clock

type Option func(*Scheduler)

func WithClock(clock Clock) Option {
	return func(s *Scheduler) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func newSystemClock() Clock {
	return clock.New()
}
