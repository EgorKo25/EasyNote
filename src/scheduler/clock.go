package scheduler

import (
	"time"

	"github.com/benbjohnson/clock"
)

type Clock = clock.Clock

type Option func(*Scheduler)

func WithClock(clock Clock) Option {
	return func(s *Scheduler) {
		s.clock = clock
	}
}

func WithReminderProcessor(processor ReminderProcessor) Option {
	return func(s *Scheduler) {
		s.processor = processor
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(s *Scheduler) {
		s.pollInterval = interval
	}
}

func newSystemClock() Clock {
	return clock.New()
}
