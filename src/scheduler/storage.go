package scheduler

import (
	"context"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

//go:generate go run go.uber.org/mock/mockgen -source=storage.go -destination=mocks/scheduler_mock.go -package=mocks
type Database interface {
	SaveReminder(ctx context.Context, reminder *database.Reminder) error
	ActualReminders(
		ctx context.Context,
		from, to time.Time,
	) ([]*database.ScheduledReminder, error)
}
