//nolint:testpackage,maintidx // Tests exercise unexported one-tick processing without adding production hooks.
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
	errCompleteOccurrence = errors.New("complete reminder")
)

func TestRunnerProcessesReadyReminders(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	pollInterval := time.Minute
	retryInterval := 5 * time.Second
	firstReminder := database.ScheduledReminder{
		ID: 1, NoteID: 10, TelegramChatID: 100, Title: "first", Timezone: "UTC",
	}
	secondReminder := database.ScheduledReminder{
		ID: 2, NoteID: 20, TelegramChatID: 200, Title: "second", Timezone: "UTC",
	}

	type expected struct {
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
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(101, nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), secondReminder).Return(102, nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), secondReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.sendDueReminders(ctx)
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
				return runner.sendDueReminders(ctx)
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
						DoAndReturn(func(context.Context, database.ScheduledReminder) (int, error) {
							close(readyForTimers)

							return 0, errSendReminder
						}).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(101, nil).Times(1),
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
					errCh <- runner.sendDueReminders(runCtx)
				}()

				<-readyForTimers

				var actualErr error

				require.Eventually(t, func() bool {
					select {
					case err := <-errCh:
						actualErr = err

						return true
					default:
						testClock.Add(retryInterval)

						return false
					}
				}, time.Second, time.Millisecond)

				return actualErr
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
						DoAndReturn(func(context.Context, database.ScheduledReminder) (int, error) {
							close(readyForTimers)

							return 0, errSendReminder
						}).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(0, errSendReminder).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusFailed, nil).
						Return(nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), secondReminder).Return(102, nil).Times(1),
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
					errCh <- runner.sendDueReminders(runCtx)
				}()

				<-readyForTimers

				var actualErr error

				require.Eventually(t, func() bool {
					select {
					case err := <-errCh:
						actualErr = err

						return true
					default:
						testClock.Add(retryInterval)

						return false
					}
				}, time.Second, time.Millisecond)

				return actualErr
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

				var actualErr error

				require.Eventually(t, func() bool {
					select {
					case <-readyForTimers:
						cancel()

						actualErr = <-errCh

						return true
					default:
						testClock.Add(pollInterval)

						return false
					}
				}, time.Second, time.Millisecond)

				return actualErr
			},
			expected: expected{expectedErrText: "wait scheduler interval: context canceled"},
		},
		{
			name: "intrusive reminder is marked delivered without completing occurrence",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        10,
				SendRetryCount:    1,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				_ *mocks.MockPlanner,
				_ chan struct{},
			) {
				intrusiveReminder := firstReminder
				intrusiveReminder.IntrusiveEnabled = true
				intrusiveReminder.IntrusiveInterval = 45 * time.Minute
				intrusiveReminder.RemindAt = baseTime

				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 10).
						Return([]*database.ScheduledReminder{&intrusiveReminder}, nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), intrusiveReminder).Return(301, nil).Times(1),
					storage.EXPECT().
						MarkReminderDelivered(gomock.Any(), database.ReminderDelivery{
							ReminderID:     intrusiveReminder.ID,
							MessageID:      301,
							NextSendAt:     baseTime.Add(45 * time.Minute),
							IntrusiveUntil: time.Date(2026, time.June, 5, 12, 0, 0, 0, time.UTC),
						}).
						Return(nil).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.sendDueReminders(ctx)
			},
		},
		{
			name: "intrusive reminder keeps one minute delivery interval",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        10,
				SendRetryCount:    1,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				_ *mocks.MockPlanner,
				_ chan struct{},
			) {
				intrusiveReminder := firstReminder
				intrusiveReminder.IntrusiveEnabled = true
				intrusiveReminder.IntrusiveInterval = time.Minute
				intrusiveReminder.RemindAt = baseTime

				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 10).
						Return([]*database.ScheduledReminder{&intrusiveReminder}, nil).
						Times(1),
					sender.EXPECT().SendReminder(gomock.Any(), intrusiveReminder).Return(302, nil).Times(1),
					storage.EXPECT().
						MarkReminderDelivered(gomock.Any(), database.ReminderDelivery{
							ReminderID:     intrusiveReminder.ID,
							MessageID:      302,
							NextSendAt:     baseTime.Add(time.Minute),
							IntrusiveUntil: time.Date(2026, time.June, 5, 12, 0, 0, 0, time.UTC),
						}).
						Return(nil).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.sendDueReminders(ctx)
			},
		},
		{
			name: "expired intrusive reminder after user local noon deletes previous message and is completed as skipped",
			cfg: config.Scheduler{
				PollInterval:      pollInterval,
				SendRetryInterval: retryInterval,
				BatchLimit:        10,
				SendRetryCount:    1,
			},
			before: func(
				_ *gomock.Controller,
				storage *mocks.MockStorage,
				sender *mocks.MockSender,
				planner *mocks.MockPlanner,
				_ chan struct{},
			) {
				expiredReminder := firstReminder
				expiredReminder.IntrusiveEnabled = true
				expiredReminder.RemindAt = time.Date(2026, time.June, 4, 18, 0, 0, 0, time.UTC)
				expiredReminder.Timezone = "Europe/Moscow"
				messageID := 777
				expiredReminder.TelegramMessageID = &messageID

				gomock.InOrder(
					storage.EXPECT().
						FindRemindersReadyToSend(gomock.Any(), baseTime, 10).
						Return([]*database.ScheduledReminder{&expiredReminder}, nil).
						Times(1),
					sender.EXPECT().
						DeleteReminderMessage(gomock.Any(), expiredReminder).
						Return(nil).
						Times(1),
					planner.EXPECT().
						CompleteOccurrence(
							gomock.Any(),
							expiredReminder.ID,
							database.ReminderStatusSkipped,
							gomock.Any(),
						).
						Return(nil).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.sendDueReminders(ctx)
			},
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
				return runner.sendDueReminders(ctx)
			},
			expected: expected{expectedErrText: "find reminders ready to send: find ready reminders"},
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
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(101, nil).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusSent, gomock.Any()).
						Return(errCompleteOccurrence).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.sendDueReminders(ctx)
			},
			expected: expected{expectedErrText: `complete reminder 1 with status "sent": complete reminder`},
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
					sender.EXPECT().SendReminder(gomock.Any(), firstReminder).Return(0, errSendReminder).Times(1),
					planner.EXPECT().
						CompleteOccurrence(gomock.Any(), firstReminder.ID, database.ReminderStatusFailed, nil).
						Return(errCompleteOccurrence).
						Times(1),
				)
			},
			run: func(ctx context.Context, runner *Runner, _ *clock.Mock, _ chan struct{}) error {
				return runner.sendDueReminders(ctx)
			},
			expected: expected{expectedErrText: `complete reminder 1 with status "failed": complete reminder`},
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
			expected: expected{expectedErrText: "wait scheduler interval: context canceled"},
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

			if testCase.expected.expectedErrText != "" {
				require.EqualErrorf(
					t,
					err,
					testCase.expected.expectedErrText,
					"case %q returned unexpected error",
					testCase.name,
				)

				return
			}

			require.NoErrorf(t, err, "case %q returned unexpected error", testCase.name)
		})
	}
}
