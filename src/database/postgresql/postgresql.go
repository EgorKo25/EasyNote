package postgresql

import (
	"context"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
)

func NewDatabase(_ config.Database) (*Database, error) {
	return &Database{}, nil
}

type Database struct {
}

func (*Database) Migrate(_ context.Context) error {
	return nil
}

func (*Database) SaveReminder(_ context.Context, _ *database.Reminder) error {
	return nil
}

func (*Database) ActualReminders(_ context.Context, _, _ time.Time) ([]*database.ScheduledReminder, error) {
	return make([]*database.ScheduledReminder, 0), nil
}
