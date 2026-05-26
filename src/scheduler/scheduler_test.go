package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/scheduler"
	"github.com/EgorKo25/EasyNote/src/scheduler/mocks"
)

var errStorageUnavailable = errors.New("storage unavailable")

func TestSchedulerSchedule(t *testing.T) {
	t.Parallel()

	reminder := scheduler.Reminder{
		TelegramUserID: 101,
		TelegramChatID: 202,
		Text:           "send report",
		RemindAt:       time.Date(2026, time.May, 26, 18, 0, 0, 0, time.UTC),
	}

	testCases := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name: "saves parsed telegram reminder",
		},
		{
			name:     "wraps database error",
			err:      errStorageUnavailable,
			expected: "save reminder: storage unavailable",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			reminderDatabase := mocks.NewMockDatabase(ctrl)
			reminderDatabase.EXPECT().
				SaveReminder(gomock.Any(), reminder).
				Return(testCase.err)

			reminderScheduler := scheduler.New(reminderDatabase)

			err := reminderScheduler.Schedule(context.Background(), reminder)

			if testCase.expected != "" {
				require.EqualError(t, err, testCase.expected)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSchedulerStartPollsDatabaseWindow(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 26, 18, 0, 0, 0, time.UTC)
	pollInterval := time.Minute
	reminders := []database.ScheduledReminder{
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
		ScheduledReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
		DoAndReturn(func(_ context.Context, _, _ time.Time) ([]database.ScheduledReminder, error) {
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

	err := reminderScheduler.Start(ctx)

	require.ErrorIs(t, err, context.Canceled)
}
