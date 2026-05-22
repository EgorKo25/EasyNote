package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

func New(reminderCache Cache, reminderDatabase Database, opts ...Option) *Scheduler {
	scheduler := &Scheduler{
		cache:    reminderCache,
		database: reminderDatabase,
		clock:    newSystemClock(),
	}

	for _, opt := range opts {
		opt(scheduler)
	}

	return scheduler
}

type Scheduler struct {
	mu sync.Mutex

	cache    Cache
	database Database
	clock    Clock
}

func (s *Scheduler) Schedule(note database.Note, reminder database.Reminder) bool {
	return s.ScheduleReminder(scheduledReminder(note, reminder))
}

func (s *Scheduler) ScheduleReminder(reminder database.ScheduledReminder) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.AddIfWithinWindow(s.clock.Now(), reminder)
}

func (s *Scheduler) Refresh(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.database == nil {
		return 0, ErrDatabaseNotConfigured
	}

	now := s.clock.Now()

	reminders, err := s.database.ScheduledReminders(ctx, now, s.cache.Window())
	if err != nil {
		return 0, fmt.Errorf("load scheduled reminders: %w", err)
	}

	for _, reminder := range reminders {
		s.cache.Upsert(reminder)
	}

	return len(reminders), nil
}

func (s *Scheduler) UpsertReminder(reminder database.ScheduledReminder) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.Upsert(reminder)
}

func (s *Scheduler) Cancel(reminderID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.Remove(reminderID)
}

func (s *Scheduler) Due() []database.ScheduledReminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.PopDue(s.clock.Now())
}

func (s *Scheduler) Next() (database.ScheduledReminder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.Next()
}

func (s *Scheduler) NextDelay() (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next, ok := s.cache.Next()
	if !ok {
		return 0, false
	}

	delay := next.RemindAt.Sub(s.clock.Now())
	if delay < 0 {
		return 0, true
	}

	return delay, true
}

func (s *Scheduler) Exists(reminderID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.Exists(reminderID)
}

func (s *Scheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.Len()
}

func (s *Scheduler) Window() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cache.Window()
}

func scheduledReminder(note database.Note, reminder database.Reminder) database.ScheduledReminder {
	return database.ScheduledReminder{
		ID:             reminder.ID,
		NoteID:         reminder.NoteID,
		TelegramChatID: note.TelegramChatID,
		Text:           note.Text,
		RemindAt:       reminder.RemindAt,
	}
}
