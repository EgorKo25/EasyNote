package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/scheduler"
	"github.com/EgorKo25/EasyNote/src/scheduler/mocks"
)

func TestSchedulerStartPollsDatabaseWindow(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 26, 18, 0, 0, 0, time.UTC)
	pollInterval := time.Minute
	reminders := []*database.ScheduledReminder{
		{ID: 1, TelegramChatID: 202, Text: "send report", RemindAt: baseTime.Add(30 * time.Second)},
	}

	ctrl := gomock.NewController(t)
	reminderDatabase := mocks.NewMockDatabase(ctrl)
	processor := mocks.NewMockReminderProcessor(ctrl)
	testClock := clock.NewMock()
	testClock.Set(baseTime)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reminderDatabase.EXPECT().
		ActualReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
		DoAndReturn(func(_ context.Context, _, _ time.Time) ([]*database.ScheduledReminder, error) {
			cancel()

			return reminders, nil
		})
	processor.EXPECT().
		ProcessReminders(gomock.Any(), reminders).
		Return(nil)

	reminderScheduler := scheduler.New(
		reminderDatabase,
		scheduler.WithClock(testClock),
		scheduler.WithPollInterval(pollInterval),
		scheduler.WithReminderProcessor(processor),
	)

	err := reminderScheduler.Run(ctx)

	require.ErrorIs(t, err, context.Canceled)
}
