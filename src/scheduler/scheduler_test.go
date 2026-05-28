package scheduler_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/scheduler"
	"github.com/EgorKo25/EasyNote/src/scheduler/mocks"
)

var errActualRemindersFailed = errors.New("actual reminders failed")

func TestSchedulerRunQueuesReminders(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 28, 10, 0, 0, 0, time.UTC)
	pollInterval := time.Minute
	firstReminder := &database.ScheduledReminder{
		ID:             1,
		NoteID:         10,
		TelegramChatID: 100,
		Text:           "first reminder",
		RemindAt:       baseTime,
	}
	secondReminder := &database.ScheduledReminder{
		ID:             2,
		NoteID:         20,
		TelegramChatID: 200,
		Text:           "second reminder",
		RemindAt:       baseTime.Add(pollInterval),
	}
	thirdReminder := &database.ScheduledReminder{
		ID:             3,
		NoteID:         30,
		TelegramChatID: 300,
		Text:           "third reminder",
		RemindAt:       baseTime.Add(30 * time.Second),
	}

	type in struct {
		baseTime              time.Time
		pollInterval          time.Duration
		cancelBeforeRun       bool
		cancelAfterNext       bool
		cancelNextBeforeRead  bool
		runScheduler          bool
		nextCalls             int
		advanceAfterFirstPoll time.Duration
		runTimeout            time.Duration
		mocks                 func(t *testing.T)
	}

	type expected struct {
		expectedError     error
		expectedNextError error
		expectedReminders []*database.ScheduledReminder
	}

	var reminderDatabase *mocks.MockDatabase
	var cancel context.CancelFunc
	var firstPollLoaded atomic.Bool

	testCases := []struct {
		name     string
		in       in
		expected expected
	}{
		{
			name: "empty polling window returns context cancellation without queued reminders",
			in: in{
				baseTime:     baseTime,
				pollInterval: pollInterval,
				runScheduler: true,
				runTimeout:   100 * time.Millisecond,
				mocks: func(t *testing.T) {
					t.Helper()

					reminderDatabase.EXPECT().
						ActualReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
						DoAndReturn(func(context.Context, time.Time, time.Time) ([]*database.ScheduledReminder, error) {
							cancel()

							return []*database.ScheduledReminder{}, nil
						})
				},
			},
			expected: expected{
				expectedError:     context.Canceled,
				expectedReminders: []*database.ScheduledReminder{},
			},
		},
		{
			name: "single reminder from current window is returned by next",
			in: in{
				baseTime:        baseTime,
				pollInterval:    pollInterval,
				cancelAfterNext: true,
				runScheduler:    true,
				nextCalls:       1,
				runTimeout:      100 * time.Millisecond,
				mocks: func(t *testing.T) {
					t.Helper()

					reminderDatabase.EXPECT().
						ActualReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
						Return([]*database.ScheduledReminder{firstReminder}, nil)
				},
			},
			expected: expected{
				expectedError:     context.Canceled,
				expectedReminders: []*database.ScheduledReminder{firstReminder},
			},
		},
		{
			name: "multiple reminders from same window are returned by next in database order",
			in: in{
				baseTime:        baseTime,
				pollInterval:    pollInterval,
				cancelAfterNext: true,
				runScheduler:    true,
				nextCalls:       3,
				runTimeout:      100 * time.Millisecond,
				mocks: func(t *testing.T) {
					t.Helper()

					reminderDatabase.EXPECT().
						ActualReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
						Return([]*database.ScheduledReminder{firstReminder, thirdReminder, secondReminder}, nil)
				},
			},
			expected: expected{
				expectedError:     context.Canceled,
				expectedReminders: []*database.ScheduledReminder{firstReminder, thirdReminder, secondReminder},
			},
		},
		{
			name: "clock advance starts next polling window and next returns reminder from second poll",
			in: in{
				baseTime:              baseTime,
				pollInterval:          pollInterval,
				cancelAfterNext:       true,
				runScheduler:          true,
				nextCalls:             1,
				advanceAfterFirstPoll: pollInterval,
				runTimeout:            200 * time.Millisecond,
				mocks: func(t *testing.T) {
					t.Helper()

					gomock.InOrder(
						reminderDatabase.EXPECT().
							ActualReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
							DoAndReturn(func(context.Context, time.Time, time.Time) ([]*database.ScheduledReminder, error) {
								firstPollLoaded.Store(true)

								return []*database.ScheduledReminder{}, nil
							}),
						reminderDatabase.EXPECT().
							ActualReminders(
								gomock.Any(),
								baseTime.Add(pollInterval),
								baseTime.Add(2*pollInterval),
							).
							Return([]*database.ScheduledReminder{secondReminder}, nil),
					)
				},
			},
			expected: expected{
				expectedError:     context.Canceled,
				expectedReminders: []*database.ScheduledReminder{secondReminder},
			},
		},
		{
			name: "scheduler continues polling when reminders are not consumed",
			in: in{
				baseTime:              baseTime,
				pollInterval:          pollInterval,
				runScheduler:          true,
				advanceAfterFirstPoll: pollInterval,
				runTimeout:            200 * time.Millisecond,
				mocks: func(t *testing.T) {
					t.Helper()

					gomock.InOrder(
						reminderDatabase.EXPECT().
							ActualReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
							DoAndReturn(func(context.Context, time.Time, time.Time) ([]*database.ScheduledReminder, error) {
								firstPollLoaded.Store(true)

								return []*database.ScheduledReminder{firstReminder}, nil
							}),
						reminderDatabase.EXPECT().
							ActualReminders(
								gomock.Any(),
								baseTime.Add(pollInterval),
								baseTime.Add(2*pollInterval),
							).
							DoAndReturn(func(context.Context, time.Time, time.Time) ([]*database.ScheduledReminder, error) {
								cancel()

								return []*database.ScheduledReminder{secondReminder}, nil
							}),
					)
				},
			},
			expected: expected{
				expectedError:     context.Canceled,
				expectedReminders: []*database.ScheduledReminder{},
			},
		},
		{
			name: "database loading error is returned without queued reminders",
			in: in{
				baseTime:     baseTime,
				pollInterval: pollInterval,
				runScheduler: true,
				runTimeout:   100 * time.Millisecond,
				mocks: func(t *testing.T) {
					t.Helper()

					reminderDatabase.EXPECT().
						ActualReminders(gomock.Any(), baseTime, baseTime.Add(pollInterval)).
						Return(nil, errActualRemindersFailed)
				},
			},
			expected: expected{
				expectedError:     errActualRemindersFailed,
				expectedReminders: []*database.ScheduledReminder{},
			},
		},
		{
			name: "context canceled before run does not poll database",
			in: in{
				baseTime:        baseTime,
				pollInterval:    pollInterval,
				cancelBeforeRun: true,
				runScheduler:    true,
				runTimeout:      100 * time.Millisecond,
			},
			expected: expected{
				expectedError:     context.Canceled,
				expectedReminders: []*database.ScheduledReminder{},
			},
		},
		{
			name: "next returns context cancellation while queue is empty",
			in: in{
				baseTime:             baseTime,
				pollInterval:         pollInterval,
				cancelNextBeforeRead: true,
				runTimeout:           100 * time.Millisecond,
			},
			expected: expected{
				expectedNextError: context.Canceled,
				expectedReminders: []*database.ScheduledReminder{},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			reminderDatabase = mocks.NewMockDatabase(ctrl)
			firstPollLoaded.Store(false)

			ctx, contextCancel := context.WithCancel(context.Background())
			cancel = contextCancel
			defer cancel()

			if testCase.in.mocks != nil {
				testCase.in.mocks(t)
			}

			testClock := clock.NewMock()
			testClock.Set(testCase.in.baseTime)
			reminderScheduler := scheduler.New(
				reminderDatabase,
				testCase.in.pollInterval,
				scheduler.WithClock(testClock),
			)

			if testCase.in.cancelBeforeRun {
				cancel()
			}

			var actualRunErr error
			runCompleted := atomic.Bool{}
			if testCase.in.runScheduler {
				go func() {
					actualRunErr = reminderScheduler.Run(ctx)
					runCompleted.Store(true)
				}()
			}

			if testCase.in.advanceAfterFirstPoll > 0 {
				require.Eventually(t, firstPollLoaded.Load, testCase.in.runTimeout, time.Millisecond)
				testClock.Add(testCase.in.advanceAfterFirstPoll)
			}

			nextCtx := context.Background()
			if testCase.in.cancelNextBeforeRead {
				cancelledCtx, cancelNext := context.WithCancel(context.Background())
				cancelNext()
				nextCtx = cancelledCtx
			}

			actualReminders := make([]*database.ScheduledReminder, 0, len(testCase.expected.expectedReminders))
			var actualNextErr error
			for range testCase.in.nextCalls {
				nextCtxWithTimeout, cancelNextTimeout := context.WithTimeout(nextCtx, testCase.in.runTimeout)
				actualReminder, err := reminderScheduler.Next(nextCtxWithTimeout)
				cancelNextTimeout()
				actualNextErr = err
				if actualNextErr != nil {
					break
				}

				actualReminders = append(actualReminders, actualReminder)
			}

			if testCase.in.cancelNextBeforeRead {
				_, actualNextErr = reminderScheduler.Next(nextCtx)
			}

			if testCase.in.cancelAfterNext {
				cancel()
			}

			if testCase.in.runScheduler {
				require.Eventually(t, runCompleted.Load, testCase.in.runTimeout, time.Millisecond)
			}

			require.Equal(t, testCase.expected.expectedReminders, actualReminders)

			if testCase.expected.expectedNextError != nil {
				require.ErrorIs(t, actualNextErr, testCase.expected.expectedNextError)
			}

			if testCase.expected.expectedError != nil {
				require.ErrorIs(t, actualRunErr, testCase.expected.expectedError)
			}
		})
	}
}
