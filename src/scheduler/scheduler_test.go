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
	"github.com/EgorKo25/EasyNote/src/database/cache"
	"github.com/EgorKo25/EasyNote/src/scheduler"
	"github.com/EgorKo25/EasyNote/src/scheduler/mocks"
)

var errStorageUnavailable = errors.New("storage unavailable")

func TestSchedulerScheduleWithRealCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name         string
		window       time.Duration
		note         database.Note
		reminder     database.Reminder
		added        bool
		next         database.ScheduledReminder
		hasNext      bool
		len          int
		nextDelay    time.Duration
		hasNextDelay bool
	}{
		{
			name:   "adds reminder inside cache window",
			window: time.Hour,
			note:   database.Note{ID: 10, TelegramChatID: 1001, Text: "send report"},
			reminder: database.Reminder{
				ID:       20,
				NoteID:   10,
				RemindAt: baseTime.Add(5 * time.Minute),
			},
			added: true,
			next: database.ScheduledReminder{
				ID:             20,
				NoteID:         10,
				TelegramChatID: 1001,
				Text:           "send report",
				RemindAt:       baseTime.Add(5 * time.Minute),
			},
			hasNext:      true,
			len:          1,
			nextDelay:    5 * time.Minute,
			hasNextDelay: true,
		},
		{
			name:   "adds reminder with unicode text",
			window: time.Hour,
			note:   database.Note{ID: 11, TelegramChatID: 1002, Text: "Купить молоко"},
			reminder: database.Reminder{
				ID:       21,
				NoteID:   11,
				RemindAt: baseTime.Add(15 * time.Minute),
			},
			added: true,
			next: database.ScheduledReminder{
				ID:             21,
				NoteID:         11,
				TelegramChatID: 1002,
				Text:           "Купить молоко",
				RemindAt:       baseTime.Add(15 * time.Minute),
			},
			hasNext:      true,
			len:          1,
			nextDelay:    15 * time.Minute,
			hasNextDelay: true,
		},
		{
			name:   "adds past reminder as due item",
			window: time.Hour,
			note:   database.Note{ID: 12, TelegramChatID: 1003, Text: "past due"},
			reminder: database.Reminder{
				ID:       22,
				NoteID:   12,
				RemindAt: baseTime.Add(-time.Minute),
			},
			added: true,
			next: database.ScheduledReminder{
				ID:             22,
				NoteID:         12,
				TelegramChatID: 1003,
				Text:           "past due",
				RemindAt:       baseTime.Add(-time.Minute),
			},
			hasNext:      true,
			len:          1,
			hasNextDelay: true,
		},
		{
			name:   "adds reminder with detached note id",
			window: time.Hour,
			note:   database.Note{ID: 13, TelegramChatID: 1004, Text: "detached"},
			reminder: database.Reminder{
				ID:       23,
				NoteID:   130,
				RemindAt: baseTime.Add(30 * time.Minute),
			},
			added: true,
			next: database.ScheduledReminder{
				ID:             23,
				NoteID:         130,
				TelegramChatID: 1004,
				Text:           "detached",
				RemindAt:       baseTime.Add(30 * time.Minute),
			},
			hasNext:      true,
			len:          1,
			nextDelay:    30 * time.Minute,
			hasNextDelay: true,
		},
		{
			name:   "adds reminder exactly on cache window boundary",
			window: 10 * time.Minute,
			note:   database.Note{ID: 14, TelegramChatID: 1005, Text: "boundary"},
			reminder: database.Reminder{
				ID:       24,
				NoteID:   14,
				RemindAt: baseTime.Add(10 * time.Minute),
			},
			added: true,
			next: database.ScheduledReminder{
				ID:             24,
				NoteID:         14,
				TelegramChatID: 1005,
				Text:           "boundary",
				RemindAt:       baseTime.Add(10 * time.Minute),
			},
			hasNext:      true,
			len:          1,
			nextDelay:    10 * time.Minute,
			hasNextDelay: true,
		},
		{
			name:   "adds reminder at current time boundary",
			window: 10 * time.Minute,
			note:   database.Note{ID: 15, TelegramChatID: 1006, Text: "now"},
			reminder: database.Reminder{
				ID:       25,
				NoteID:   15,
				RemindAt: baseTime,
			},
			added: true,
			next: database.ScheduledReminder{
				ID:             25,
				NoteID:         15,
				TelegramChatID: 1006,
				Text:           "now",
				RemindAt:       baseTime,
			},
			hasNext:      true,
			len:          1,
			hasNextDelay: true,
		},
		{
			name:   "rejects reminder just outside cache window",
			window: 10 * time.Minute,
			note:   database.Note{ID: 16, TelegramChatID: 1007, Text: "too late"},
			reminder: database.Reminder{
				ID:       26,
				NoteID:   16,
				RemindAt: baseTime.Add(10*time.Minute + time.Nanosecond),
			},
		},
		{
			name:   "rejects far future reminder",
			window: 10 * time.Minute,
			note:   database.Note{ID: 17, TelegramChatID: 1008, Text: "tomorrow"},
			reminder: database.Reminder{
				ID:       27,
				NoteID:   17,
				RemindAt: baseTime.Add(24 * time.Hour),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testClock := clock.NewMock()
			testClock.Set(baseTime)
			reminderScheduler := scheduler.New(
				cache.NewReminderCache(testCase.window),
				nil,
				scheduler.WithClock(testClock),
			)

			added := reminderScheduler.Schedule(testCase.note, testCase.reminder)
			next, hasNext := reminderScheduler.Next()
			delay, hasDelay := reminderScheduler.NextDelay()

			require.Equal(t, testCase.added, added)
			require.Equal(t, testCase.hasNext, hasNext)
			require.Equal(t, testCase.next, clearReminderIndex(next))
			require.Equal(t, testCase.len, reminderScheduler.Len())
			require.Equal(t, testCase.hasNextDelay, hasDelay)
			require.Equal(t, testCase.nextDelay, delay)
		})
	}
}

func TestSchedulerDueWithRealCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name    string
		advance time.Duration
		seed    []database.ScheduledReminder
		due     []database.ScheduledReminder
		next    database.ScheduledReminder
		hasNext bool
		len     int
	}{
		{
			name:    "returns reminder after clock reaches due time",
			advance: 5 * time.Minute,
			seed:    []database.ScheduledReminder{{ID: 1, NoteID: 10, RemindAt: baseTime.Add(5 * time.Minute)}},
			due:     []database.ScheduledReminder{{ID: 1, NoteID: 10, RemindAt: baseTime.Add(5 * time.Minute)}},
		},
		{
			name:    "returns multiple due reminders",
			advance: 5 * time.Minute,
			seed: []database.ScheduledReminder{
				{ID: 2, NoteID: 20, RemindAt: baseTime.Add(2 * time.Minute)},
				{ID: 3, NoteID: 30, RemindAt: baseTime.Add(5 * time.Minute)},
			},
			due: []database.ScheduledReminder{
				{ID: 2, NoteID: 20, RemindAt: baseTime.Add(2 * time.Minute)},
				{ID: 3, NoteID: 30, RemindAt: baseTime.Add(5 * time.Minute)},
			},
		},
		{
			name: "returns reminder that is already in the past",
			seed: []database.ScheduledReminder{
				{ID: 4, NoteID: 40, RemindAt: baseTime.Add(-time.Minute)},
			},
			due: []database.ScheduledReminder{{ID: 4, NoteID: 40, RemindAt: baseTime.Add(-time.Minute)}},
		},
		{
			name: "returns reminder exactly at current time boundary",
			seed: []database.ScheduledReminder{{ID: 5, NoteID: 50, RemindAt: baseTime}},
			due:  []database.ScheduledReminder{{ID: 5, NoteID: 50, RemindAt: baseTime}},
		},
		{
			name:    "keeps reminder just before due boundary",
			advance: 5*time.Minute - time.Nanosecond,
			seed:    []database.ScheduledReminder{{ID: 6, NoteID: 60, RemindAt: baseTime.Add(5 * time.Minute)}},
			next:    database.ScheduledReminder{ID: 6, NoteID: 60, RemindAt: baseTime.Add(5 * time.Minute)},
			hasNext: true,
			len:     1,
		},
		{
			name:    "keeps later reminder after due prefix is removed",
			advance: time.Minute,
			seed: []database.ScheduledReminder{
				{ID: 7, NoteID: 70, RemindAt: baseTime.Add(time.Minute)},
				{ID: 8, NoteID: 80, RemindAt: baseTime.Add(2 * time.Minute)},
			},
			due:     []database.ScheduledReminder{{ID: 7, NoteID: 70, RemindAt: baseTime.Add(time.Minute)}},
			next:    database.ScheduledReminder{ID: 8, NoteID: 80, RemindAt: baseTime.Add(2 * time.Minute)},
			hasNext: true,
			len:     1,
		},
		{
			name: "returns empty due list on empty scheduler",
		},
		{
			name:    "returns empty due list when all reminders are in future",
			seed:    []database.ScheduledReminder{{ID: 10, NoteID: 100, RemindAt: baseTime.Add(time.Minute)}},
			next:    database.ScheduledReminder{ID: 10, NoteID: 100, RemindAt: baseTime.Add(time.Minute)},
			hasNext: true,
			len:     1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testClock := clock.NewMock()
			testClock.Set(baseTime)
			reminderScheduler := scheduler.New(
				cache.NewReminderCache(24*time.Hour),
				nil,
				scheduler.WithClock(testClock),
			)

			for _, reminder := range testCase.seed {
				reminderScheduler.UpsertReminder(reminder)
			}

			testClock.Add(testCase.advance)

			due := reminderScheduler.Due()
			next, hasNext := reminderScheduler.Next()

			require.ElementsMatch(t, testCase.due, clearReminderIndexes(due))
			require.Equal(t, testCase.hasNext, hasNext)
			require.Equal(t, testCase.next, clearReminderIndex(next))
			require.Equal(t, testCase.len, reminderScheduler.Len())
		})
	}
}

func TestSchedulerRefreshWithMockDatabase(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name      string
		now       time.Time
		window    time.Duration
		reminders []database.ScheduledReminder
		err       error
		errText   string
		loaded    int
		next      database.ScheduledReminder
		hasNext   bool
		len       int
	}{
		{
			name:      "loads one reminder from database",
			now:       baseTime,
			window:    time.Hour,
			reminders: []database.ScheduledReminder{{ID: 1, NoteID: 10, RemindAt: baseTime.Add(time.Minute)}},
			loaded:    1,
			next:      database.ScheduledReminder{ID: 1, NoteID: 10, RemindAt: baseTime.Add(time.Minute)},
			hasNext:   true,
			len:       1,
		},
		{
			name:   "loads multiple reminders from database",
			now:    baseTime,
			window: time.Hour,
			reminders: []database.ScheduledReminder{
				{ID: 2, NoteID: 20, RemindAt: baseTime.Add(2 * time.Minute)},
				{ID: 3, NoteID: 30, RemindAt: baseTime.Add(time.Minute)},
			},
			loaded:  2,
			next:    database.ScheduledReminder{ID: 3, NoteID: 30, RemindAt: baseTime.Add(time.Minute)},
			hasNext: true,
			len:     2,
		},
		{
			name:      "loads reminder with text payload",
			now:       baseTime,
			window:    time.Hour,
			reminders: []database.ScheduledReminder{{ID: 4, Text: "payload", RemindAt: baseTime}},
			loaded:    1,
			next:      database.ScheduledReminder{ID: 4, Text: "payload", RemindAt: baseTime},
			hasNext:   true,
			len:       1,
		},
		{
			name:      "loads reminder using non default window",
			now:       baseTime,
			window:    15 * time.Minute,
			reminders: []database.ScheduledReminder{{ID: 5, RemindAt: baseTime.Add(15 * time.Minute)}},
			loaded:    1,
			next:      database.ScheduledReminder{ID: 5, RemindAt: baseTime.Add(15 * time.Minute)},
			hasNext:   true,
			len:       1,
		},
		{
			name:   "loads no reminders on empty database result boundary",
			now:    baseTime,
			window: time.Hour,
		},
		{
			name:      "passes advanced clock time into database boundary",
			now:       baseTime.Add(time.Hour),
			window:    time.Hour,
			reminders: []database.ScheduledReminder{{ID: 6, RemindAt: baseTime.Add(time.Hour)}},
			loaded:    1,
			next:      database.ScheduledReminder{ID: 6, RemindAt: baseTime.Add(time.Hour)},
			hasNext:   true,
			len:       1,
		},
		{
			name:    "wraps database error",
			now:     baseTime,
			window:  time.Hour,
			err:     errStorageUnavailable,
			errText: "load scheduled reminders: storage unavailable",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			reminderDatabase := mocks.NewMockDatabase(ctrl)
			reminderDatabase.EXPECT().
				ScheduledReminders(gomock.Any(), testCase.now, testCase.window).
				Return(testCase.reminders, testCase.err)

			testClock := clock.NewMock()
			testClock.Set(testCase.now)
			reminderScheduler := scheduler.New(
				cache.NewReminderCache(testCase.window),
				reminderDatabase,
				scheduler.WithClock(testClock),
			)

			loaded, err := reminderScheduler.Refresh(context.Background())
			next, hasNext := reminderScheduler.Next()

			if testCase.errText != "" {
				require.EqualErrorf(t, err, testCase.errText, "unexpected refresh error")
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, testCase.loaded, loaded)
			require.Equal(t, testCase.hasNext, hasNext)
			require.Equal(t, testCase.next, clearReminderIndex(next))
			require.Equal(t, testCase.len, reminderScheduler.Len())
		})
	}
}

func TestSchedulerRefreshWithoutDatabase(t *testing.T) {
	t.Parallel()

	testClock := clock.NewMock()
	testClock.Set(time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC))
	reminderScheduler := scheduler.New(
		cache.NewReminderCache(time.Hour),
		nil,
		scheduler.WithClock(testClock),
	)

	loaded, err := reminderScheduler.Refresh(context.Background())

	require.EqualErrorf(t, err, "scheduler database is not configured", "unexpected refresh error")
	require.Equal(t, 0, loaded)
}

func TestSchedulerWithMockCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name   string
		assert func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache)
	}{
		{
			name: "schedule reminder passes clock time and reminder to cache",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminder := database.ScheduledReminder{ID: 1, RemindAt: baseTime.Add(time.Minute)}
				reminderCache.EXPECT().AddIfWithinWindow(baseTime, reminder).Return(true)

				require.True(t, reminderScheduler.ScheduleReminder(reminder))
			},
		},
		{
			name: "due passes current clock time to cache",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminders := []database.ScheduledReminder{{ID: 2, RemindAt: baseTime}}
				reminderCache.EXPECT().PopDue(baseTime).Return(reminders)

				require.Equal(t, reminders, reminderScheduler.Due())
			},
		},
		{
			name: "cancel delegates reminder id to cache",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminderCache.EXPECT().Remove(int64(3)).Return(true)

				require.True(t, reminderScheduler.Cancel(3))
			},
		},
		{
			name: "next delay returns false when cache has no next reminder",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminderCache.EXPECT().Next().Return(database.ScheduledReminder{}, false)

				delay, ok := reminderScheduler.NextDelay()

				require.False(t, ok)
				require.Equal(t, time.Duration(0), delay)
			},
		},
		{
			name: "next delay returns future duration from cache reminder",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminder := database.ScheduledReminder{ID: 4, RemindAt: baseTime.Add(5 * time.Minute)}
				reminderCache.EXPECT().Next().Return(reminder, true)

				delay, ok := reminderScheduler.NextDelay()

				require.True(t, ok)
				require.Equal(t, 5*time.Minute, delay)
			},
		},
		{
			name: "next delay returns zero for overdue cache reminder",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminder := database.ScheduledReminder{ID: 5, RemindAt: baseTime.Add(-time.Minute)}
				reminderCache.EXPECT().Next().Return(reminder, true)

				delay, ok := reminderScheduler.NextDelay()

				require.True(t, ok)
				require.Equal(t, time.Duration(0), delay)
			},
		},
		{
			name: "window delegates to cache",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminderCache.EXPECT().Window().Return(time.Hour)

				require.Equal(t, time.Hour, reminderScheduler.Window())
			},
		},
		{
			name: "len delegates to cache",
			assert: func(t *testing.T, reminderScheduler *scheduler.Scheduler, reminderCache *mocks.MockCache) {
				t.Helper()

				reminderCache.EXPECT().Len().Return(2)

				require.Equal(t, 2, reminderScheduler.Len())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			reminderCache := mocks.NewMockCache(ctrl)
			testClock := clock.NewMock()
			testClock.Set(baseTime)
			reminderScheduler := scheduler.New(reminderCache, nil, scheduler.WithClock(testClock))

			testCase.assert(t, reminderScheduler, reminderCache)
		})
	}
}

func TestSchedulerLoadWithRealCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name          string
		reminderCount int
		window        time.Duration
		dueAt         time.Duration
		expectedDue   int
		expectedRest  int
	}{
		{
			name:          "loads one thousand reminders and drains all of them",
			reminderCount: 1_000,
			window:        24 * time.Hour,
			dueAt:         24 * time.Hour,
			expectedDue:   1_000,
		},
		{
			name:          "loads ten thousand reminders and drains all of them",
			reminderCount: 10_000,
			window:        24 * time.Hour,
			dueAt:         24 * time.Hour,
			expectedDue:   10_000,
		},
		{
			name:          "keeps half of ten thousand reminders outside due point",
			reminderCount: 10_000,
			window:        24 * time.Hour,
			dueAt:         12 * time.Hour,
			expectedDue:   5_001,
			expectedRest:  4_999,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testClock := clock.NewMock()
			testClock.Set(baseTime)
			reminderScheduler := scheduler.New(
				cache.NewReminderCache(testCase.window),
				nil,
				scheduler.WithClock(testClock),
			)

			for index := range testCase.reminderCount {
				reminderID := int64(index + 1)
				reminderOffset := time.Duration(index%testCase.reminderCount) * testCase.window /
					time.Duration(testCase.reminderCount)

				added := reminderScheduler.ScheduleReminder(database.ScheduledReminder{
					ID:       reminderID,
					NoteID:   reminderID,
					RemindAt: baseTime.Add(reminderOffset),
				})

				require.True(t, added)
			}

			testClock.Add(testCase.dueAt)

			due := reminderScheduler.Due()

			require.Len(t, due, testCase.expectedDue)
			require.Equal(t, testCase.expectedRest, reminderScheduler.Len())
		})
	}
}

func BenchmarkSchedulerScheduleAndDrainWithRealCache(b *testing.B) {
	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	for range b.N {
		testClock := clock.NewMock()
		testClock.Set(baseTime)
		reminderScheduler := scheduler.New(
			cache.NewReminderCache(24*time.Hour),
			nil,
			scheduler.WithClock(testClock),
		)

		for index := range 10_000 {
			reminderID := int64(index + 1)

			reminderScheduler.ScheduleReminder(database.ScheduledReminder{
				ID:       reminderID,
				NoteID:   reminderID,
				RemindAt: baseTime.Add(time.Duration(index) * time.Second),
			})
		}

		testClock.Add(24 * time.Hour)
		reminderScheduler.Due()
	}
}

func clearReminderIndexes(reminders []database.ScheduledReminder) []database.ScheduledReminder {
	cleared := make([]database.ScheduledReminder, 0, len(reminders))

	for _, reminder := range reminders {
		cleared = append(cleared, clearReminderIndex(reminder))
	}

	return cleared
}

func clearReminderIndex(reminder database.ScheduledReminder) database.ScheduledReminder {
	reminder.Index = 0

	return reminder
}
