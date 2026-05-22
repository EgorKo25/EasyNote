package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

var ErrDatabaseNotConfigured = errors.New("scheduler database is not configured")

//go:generate go run go.uber.org/mock/mockgen -source=dependencies.go -destination=mocks/scheduler_mock.go -package=mocks
type Cache interface {
	AddIfWithinWindow(now time.Time, reminder database.ScheduledReminder) bool
	Upsert(reminder database.ScheduledReminder)
	Remove(reminderID int64) bool
	PopDue(now time.Time) []database.ScheduledReminder
	Next() (database.ScheduledReminder, bool)
	Exists(reminderID int64) bool
	Len() int
	Window() time.Duration
}

type Database interface {
	ScheduledReminders(
		ctx context.Context,
		now time.Time,
		window time.Duration,
	) ([]database.ScheduledReminder, error)
}
