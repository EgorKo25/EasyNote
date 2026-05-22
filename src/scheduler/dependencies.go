package scheduler

import (
	"context"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

//go:generate go run go.uber.org/mock/mockgen -source=dependencies.go -destination=mocks/scheduler_mock.go -package=mocks
type Cache interface {
	AddIfWithinWindow(now time.Time, reminder database.ScheduledReminder) bool
	Replace(reminders []database.ScheduledReminder)
	Upsert(reminder database.ScheduledReminder)
	Remove(reminderID int64) bool
	PopDue(now time.Time) []database.ScheduledReminder
	Next() (database.ScheduledReminder, bool)
	Exists(reminderID int64) bool
	Len() int
	Window() time.Duration
}

type Database interface {
	SaveScheduledReminder(ctx context.Context, reminder database.ScheduledReminder) error
	ScheduledReminders(
		ctx context.Context,
		from time.Time,
		to time.Time,
	) ([]database.ScheduledReminder, error)
}
