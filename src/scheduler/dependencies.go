package scheduler

import (
	"context"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

//go:generate go run go.uber.org/mock/mockgen -source=dependencies.go -destination=mocks/scheduler_mock.go -package=mocks
type Storage interface {
	FindRemindersReadyToSend(
		ctx context.Context,
		now time.Time,
		limit int,
	) ([]*database.ScheduledReminder, error)
}

type Sender interface {
	SendReminder(ctx context.Context, reminder database.ScheduledReminder) error
}

type Planner interface {
	CompleteOccurrence(
		ctx context.Context,
		reminderID int64,
		status database.ReminderStatus,
		sentAt *time.Time,
	) error
}

type Retrier interface {
	Retry(ctx context.Context, operation func() error, notify func(error, time.Duration)) error
}
