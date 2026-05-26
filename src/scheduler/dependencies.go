package scheduler

import (
	"context"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

//go:generate go run go.uber.org/mock/mockgen -source=dependencies.go -destination=mocks/scheduler_mock.go -package=mocks
type Database interface {
	SaveReminder(ctx context.Context, reminder Reminder) error
	ScheduledReminders(
		ctx context.Context,
		from time.Time,
		to time.Time,
	) ([]database.ScheduledReminder, error)
}

type ReminderProcessor interface {
	ProcessReminders(ctx context.Context, reminders []database.ScheduledReminder) error
}
