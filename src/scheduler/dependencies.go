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
	FindHolidaysReadyToSend(
		ctx context.Context,
		now time.Time,
		limit int,
	) ([]*database.ScheduledHoliday, error)
	MarkReminderDelivered(ctx context.Context, delivery database.ReminderDelivery) error
}

type Sender interface {
	SendReminder(ctx context.Context, reminder database.ScheduledReminder) (int, error)
	SendHoliday(ctx context.Context, holiday database.ScheduledHoliday) (int, error)
	DeleteReminderMessage(ctx context.Context, reminder database.ScheduledReminder) error
}

type Planner interface {
	CompleteOccurrence(
		ctx context.Context,
		reminderID int64,
		status database.ReminderStatus,
		sentAt *time.Time,
	) error
	CompleteHoliday(ctx context.Context, holidayID int64, sentAt *time.Time) error
}

type Retrier interface {
	Retry(ctx context.Context, operation func() error, notify func(error, time.Duration)) error
}
