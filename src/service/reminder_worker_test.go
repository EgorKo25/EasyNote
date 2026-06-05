package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/service/mocks"
)

var errSendReminder = errors.New("send reminder")

func TestReminderWorkerSendsReadyReminders(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	firstReminder := database.ScheduledReminder{ID: 1, NoteID: 10, TelegramChatID: 100, Title: "first"}
	secondReminder := database.ScheduledReminder{ID: 2, NoteID: 20, TelegramChatID: 200, Title: "second"}
	repository := mocks.NewMockReminderRepository(ctrl)
	sender := mocks.NewMockReminderSender(ctrl)
	planner := mocks.NewMockReminderCompletionPlanner(ctrl)
	worker := service.NewReminderWorker(repository, sender, planner, config.Scheduler{
		PollInterval: time.Nanosecond,
		BatchLimit:   50,
		MaxAttempts:  5,
	})

	gomock.InOrder(
		repository.EXPECT().
			FindRemindersReadyToSend(gomock.Any(), gomock.Any(), 50, 5).
			Return([]*database.ScheduledReminder{&firstReminder, &secondReminder}, nil).
			Times(1),
		sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(nil).Times(1),
		planner.EXPECT().
			CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusSent, gomock.Any()).
			Return(nil).
			Times(1),
		sender.EXPECT().
			SendReminder(gomock.Any(), secondReminder).
			DoAndReturn(func(context.Context, database.ScheduledReminder) error {
				cancel()

				return nil
			}).
			Times(1),
		planner.EXPECT().
			CompleteOccurrence(gomock.Any(), secondReminder.ID, database.ReminderStatusSent, gomock.Any()).
			Return(nil).
			Times(1),
	)

	err := worker.Run(ctx)

	require.ErrorIs(t, err, context.Canceled)
}

func TestReminderWorkerDoesNothingWhenNoRemindersAreReady(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	repository := mocks.NewMockReminderRepository(ctrl)
	sender := mocks.NewMockReminderSender(ctrl)
	planner := mocks.NewMockReminderCompletionPlanner(ctrl)
	worker := service.NewReminderWorker(repository, sender, planner, config.Scheduler{
		PollInterval: time.Nanosecond,
		BatchLimit:   50,
		MaxAttempts:  5,
	})

	repository.EXPECT().
		FindRemindersReadyToSend(gomock.Any(), gomock.Any(), 50, 5).
		DoAndReturn(func(context.Context, time.Time, int, int) ([]*database.ScheduledReminder, error) {
			cancel()

			return []*database.ScheduledReminder{}, nil
		}).
		Times(1)

	err := worker.Run(ctx)

	require.ErrorIs(t, err, context.Canceled)
}

func TestReminderWorkerMarksFailedReminderAfterSendError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	reminder := database.ScheduledReminder{ID: 7, NoteID: 70, TelegramChatID: 700, Title: "fail"}
	repository := mocks.NewMockReminderRepository(ctrl)
	sender := mocks.NewMockReminderSender(ctrl)
	planner := mocks.NewMockReminderCompletionPlanner(ctrl)
	worker := service.NewReminderWorker(repository, sender, planner, config.Scheduler{
		PollInterval: time.Nanosecond,
		BatchLimit:   50,
		MaxAttempts:  5,
	})

	gomock.InOrder(
		repository.EXPECT().
			FindRemindersReadyToSend(gomock.Any(), gomock.Any(), 50, 5).
			Return([]*database.ScheduledReminder{&reminder}, nil).
			Times(1),
		sender.EXPECT().
			SendReminder(gomock.Any(), reminder).
			DoAndReturn(func(context.Context, database.ScheduledReminder) error {
				cancel()

				return errSendReminder
			}).
			Times(1),
		planner.EXPECT().
			CompleteOccurrence(gomock.Any(), reminder.ID, database.ReminderStatusFailed, nil).
			Return(nil).
			Times(1),
	)

	err := worker.Run(ctx)

	require.ErrorIs(t, err, context.Canceled)
}

func TestReminderWorkerStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	worker := service.NewReminderWorker(
		mocks.NewMockReminderRepository(ctrl),
		mocks.NewMockReminderSender(ctrl),
		mocks.NewMockReminderCompletionPlanner(ctrl),
		config.Scheduler{
			PollInterval: time.Hour,
			BatchLimit:   50,
			MaxAttempts:  5,
		},
	)

	err := worker.Run(ctx)

	require.ErrorIs(t, err, context.Canceled)
}
