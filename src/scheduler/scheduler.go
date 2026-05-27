package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

func New(reminderDatabase Database, opts ...Option) *Scheduler {
	scheduler := &Scheduler{
		database:     reminderDatabase,
		clock:        newSystemClock(),
		pollInterval: time.Minute,
	}

	for _, opt := range opts {
		opt(scheduler)
	}

	return scheduler
}

type Scheduler struct {
	mu sync.Mutex

	database Database
	clock    Clock

	processor    ReminderProcessor
	pollInterval time.Duration
}

func (s *Scheduler) Info() string {
	return fmt.Sprintf("runed scheduler, with poll interval: %s", s.pollInterval)
}

func (s *Scheduler) Schedule(ctx context.Context, reminder *database.Reminder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.database.SaveReminder(ctx, reminder); err != nil {
		return fmt.Errorf("save reminder: %w", err)
	}

	return nil
}
