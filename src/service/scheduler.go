package service

import (
	"context"

	"github.com/EgorKo25/EasyNote/src/database"
)

type Scheduler interface {
	Noter
	Reminder
}

type Noter interface {
	AddNote(ctx context.Context, note database.Note) (database.Note, error)
	DeleteNote(ctx context.Context, noteID int64) error
	GetAllNotes(ctx context.Context) ([]database.Note, error)
}

type Reminder interface {
	AddReminder(ctx context.Context, reminder database.Reminder) (database.Reminder, error)
	DeleteReminder(ctx context.Context, reminderID int64) error
	GetAllReminders(ctx context.Context) ([]database.Reminder, error)
}
