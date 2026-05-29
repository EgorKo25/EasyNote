package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

func New(reminderDatabase Database, pollInterval time.Duration, opts ...Option) *Scheduler {
	scheduler := &Scheduler{
		database:     reminderDatabase,
		clock:        newSystemClock(),
		pollInterval: pollInterval,
	}
	scheduler.queueReady = sync.NewCond(&scheduler.mu)

	for _, opt := range opts {
		opt(scheduler)
	}

	return scheduler
}

type Scheduler struct {
	mu sync.Mutex

	database Database
	clock    Clock

	pollInterval time.Duration
	queue        []*database.ScheduledReminder
	queueReady   *sync.Cond
}

func (s *Scheduler) Info() string {
	return fmt.Sprintf("runed scheduler, with poll interval: %s", s.pollInterval)
}

func (s *Scheduler) Next(ctx context.Context) (*database.ScheduledReminder, error) {
	stopContextWatch := context.AfterFunc(ctx, func() {
		s.queueReady.Broadcast()
	})
	defer stopContextWatch()

	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.queue) == 0 {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wait next scheduled reminder: %w", err)
		}

		s.queueReady.Wait()
	}

	reminder := s.queue[0]
	s.queue = s.queue[1:]

	return reminder, nil
}
