//nolint:testpackage,maintidx,cyclop // Tests exercise unexported one-tick processing without adding production hooks.
package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/scheduler/mocks"
)

var (
	errFindReadyReminders = errors.New("find ready reminders")
	errSendReminder       = errors.New("send reminder")
	errCompleteReminder   = errors.New("complete reminder")
)

func TestRunnerProcessesReadyReminders(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	pollInterval := time.Minute
	retryInterval := 5 * time.Second
	firstReminder := database.ScheduledReminder{ID: 1, NoteID: 10, TelegramChatID: 100, Title: "first"}
	secondReminder := database.ScheduledReminder{ID: 2, NoteID: 20, TelegramChatID: 200, Title: "second"}

	type expected struct {
		expectedErrIs   error
		expectedErrText string
	}

	testCases := []struct {
		name           string
		cfg            config.Scheduler
		before         func(*gomock.Controller, *mocks.MockStorage, *mocks.MockSender, *mocks.MockPlanner, chan struct{})
		run            func(context.Context, *Runner, *clock.Mock, chan struct{}) error
		readyForTimers chan struct{}
		expected       expected
	}{
		{
			name: "ready reminders are loaded sent and completed as sent",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        50,
				SendRetryCount:    3,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				planner *mocks.MockPlanner,
				_ chan struct{},
			) {
				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 50).
						Return([]*database.ScheduledReminder{&firstReminder, &secondReminder}, nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), secondReminder).Return(nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), secondReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.processReadyReminders(ctx)
			},
		},
		{
			name: "empty ready reminder batch does nothing",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        25,
				SendRetryCount:    2,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				_ *mocks.MockSender,
				_ *mocks.MockPlanner,
				_ chan struct{},
			) {
				storage.EXPECT().
					FindRemindersReadyToSend(gomock.Any(), baseTime, 25).
					Return([]*database.ScheduledReminder{}, nil).
					Times(1)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.processReadyReminders(ctx)
			},
		},
		{
			name: "send retry succeeds and occurrence is completed as sent",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        10,
				SendRetryCount:    2,
			},
			readyForTimers: make(chan struct{}),
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				planner *mocks.MockPlanner,
				readyForTimers chan struct{},
			) {
				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 10).
						Return([]*database.ScheduledReminder{&firstReminder}, nil).
						Times(1),
					sender.EXPECT().
						SendReminder(gomock.Any(), firstReminder).
						DoAndReturn(func(context.Context, database.ScheduledReminder) error {
							close(readyForTimers)

							return errSendReminder
						}).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, testClock *clock.Mock, readyForTimers chan struct{}) error {
				runCtx, cancel := context.WithCancel(ctx)
				defer cancel()

				errCh := make(chan error, 1)

				go func() {
					errCh <- runner.processReadyReminders(runCtx)
				}()

				<-readyForTimers

				for range 10 {
					select {
					case err := <-errCh:
						return err
					default:
						testClock.Add(retryInterval)
					}
				}

				cancel()

				return <-errCh
			},
		},
		{
			name: "failed reminder is completed as failed and next reminder is still processed",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        10,
				SendRetryCount:    2,
			},
			readyForTimers: make(chan struct{}),
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				planner *mocks.MockPlanner,
				readyForTimers chan struct{},
			) {
				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 10).
						Return([]*database.ScheduledReminder{&firstReminder, &secondReminder}, nil).
						Times(1),
					sender.EXPECT().
						SendReminder(gomock.Any(), firstReminder).
						DoAndReturn(func(context.Context, database.ScheduledReminder) error {
							close(readyForTimers)

							return errSendReminder
						}).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(errSendReminder).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusFailed, nil).
						Return(nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), secondReminder).Return(nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), secondReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, testClock *clock.Mock, readyForTimers chan struct{}) error {
				runCtx, cancel := context.WithCancel(ctx)
				defer cancel()

				errCh := make(chan error, 1)

				go func() {
					errCh <- runner.processReadyReminders(runCtx)
				}()

				<-readyForTimers

				for range 10 {
					select {
					case err := <-errCh:
						return err
					default:
						testClock.Add(retryInterval)
					}
				}

				cancel()

				return <-errCh
			},
		},
		{
			name: "run waits for polling tick from clock before loading reminders",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        5,
				SendRetryCount:    1,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				_ *mocks.MockSender,
				_ *mocks.MockPlanner,
				readyForTimers chan struct{},
			) {
				storage.EXPECT().
					FindRemindersReadyToSend(gomock.Any(), gomock.Any(), 5).
					DoAndReturn(func(context.Context, time.Time, int) ([]*database.ScheduledReminder, error) {
						close(readyForTimers)

						return []*database.ScheduledReminder{}, nil
					}).
					Times(1)
			},
			readyForTimers: make(chan struct{}),
			run: func(ctx context.Context, runner *Runner, testClock *clock.Mock, readyForTimers chan struct{}) error {
				runCtx, cancel := context.WithCancel(ctx)
				errCh := make(chan error, 1)

				go func() {
					errCh <- runner.Run(runCtx)
				}()

				for range 10 {
					select {
					case <-readyForTimers:
						cancel()

						return <-errCh
					default:
						testClock.Add(pollInterval)
					}
				}

				cancel()

				return <-errCh
			},
			expected: expected{expectedErrIs: context.Canceled},
		},
		{
			name: "storage loading error is returned",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        50,
				SendRetryCount:    3,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				_ *mocks.MockSender,
				_ *mocks.MockPlanner,
				_ chan struct{},
			) {
				storage.EXPECT().
					FindRemindersReadyToSend(gomock.Any(), baseTime, 50).
					Return(nil, errFindReadyReminders).
					Times(1)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.processReadyReminders(ctx)
			},
			expected: expected{expectedErrIs: errFindReadyReminders},
		},
		{
			name: "planner sent completion error is returned",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        50,
				SendRetryCount:    1,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				planner *mocks.MockPlanner,
				_ chan struct{},
			) {
				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 50).
						Return([]*database.ScheduledReminder{&firstReminder}, nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(errCompleteReminder).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.processReadyReminders(ctx)
			},
			expected: expected{expectedErrIs: errCompleteReminder},
		},
		{
			name: "planner failed completion error is returned after exhausted retries",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        50,
				SendRetryCount:    1,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				planner *mocks.MockPlanner,
				_ chan struct{},
			) {
				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 50).
						Return([]*database.ScheduledReminder{&firstReminder}, nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(errSendReminder).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusFailed, nil).
						Return(errCompleteReminder).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.processReadyReminders(ctx)
			},
			expected: expected{expectedErrIs: errCompleteReminder},
		},
		{
			name: "context cancellation before run is returned without loading reminders",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        50,
				SendRetryCount:    3,
			},
			before: func(
				_ *gomock.Controller,
				_ *mocks.MockStorage,
				_ *mocks.MockSender,
				_ *mocks.MockPlanner,
				_ chan struct{},
			) {
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				runCtx, cancel := context.WithCancel(ctx)
				cancel()

				return runner.Run(runCtx)
			},
			expected: expected{expectedErrIs: context.Canceled},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			storage := mocks.NewMockStorage(ctrl)
			sender := mocks.NewMockSender(ctrl)
			planner := mocks.NewMockPlanner(ctrl)
			testClock := clock.NewMock()
			testClock.Set(baseTime)

			if testCase.before != nil {
				testCase.before(ctrl, storage, sender, planner, testCase.readyForTimers)
			}

			runner := NewRunner(
				storage,
				sender,
				planner,
				testCase.cfg,
				WithClock(testClock),
			)

			err := testCase.run(t.Context(), runner, testClock, testCase.readyForTimers)

			if testCase.expected.expectedErrIs != nil {
				require.ErrorIs(t, err, testCase.expected.expectedErrIs)

				return
			}

			if testCase.expected.expectedErrText != "" {
				require.EqualError(t, err, testCase.expected.expectedErrText)

				return
			}

			require.NoError(t, err)
		})
	}
}
