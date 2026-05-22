package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

type scheduleExpectation struct {
	added        bool
	errText      string
	next         database.ScheduledReminder
	hasNext      bool
	len          int
	nextDelay    time.Duration
	hasNextDelay bool
}

type dueExpectation struct {
	due     []database.ScheduledReminder
	next    database.ScheduledReminder
	hasNext bool
	len     int
}

type refreshExpectation struct {
	errText string
	loaded  int
	next    database.ScheduledReminder
	hasNext bool
	len     int
	exists  bool
}

type loadExpectation struct {
	due  int
	rest int
}

type concurrentExpectation struct {
	len   int
	added int64
}

func TestSchedulerScheduleWithRealCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		window   time.Duration
		note     database.Note
		reminder database.Reminder
		err      error
		expected scheduleExpectation
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
			expected: scheduleExpectation{
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
			expected: scheduleExpectation{
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
			expected: scheduleExpectation{
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
			expected: scheduleExpectation{
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
			expected: scheduleExpectation{
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
			expected: scheduleExpectation{
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
			expected: scheduleExpectation{
				added: false,
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
			expected: scheduleExpectation{
				added: false,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testClock := clock.NewMock()
			testClock.Set(baseTime)

			ctrl := gomock.NewController(t)
			reminderDatabase := mocks.NewMockDatabase(ctrl)
			expectedSavedReminder := expectedScheduledReminder(testCase.note, testCase.reminder)
			reminderDatabase.EXPECT().
				SaveScheduledReminder(gomock.Any(), expectedSavedReminder).
				Return(testCase.err)

			reminderScheduler := scheduler.New(
				cache.NewReminderCache(testCase.window),
				reminderDatabase,
				scheduler.WithClock(testClock),
			)

			added, err := reminderScheduler.Schedule(context.Background(), testCase.note, testCase.reminder)
			next, hasNext := reminderScheduler.Next()
			delay, hasDelay := reminderScheduler.NextDelay()

			if testCase.expected.errText != "" {
				require.EqualErrorf(t, err, testCase.expected.errText, "unexpected schedule error")
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, testCase.expected.added, added)
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.next, clearReminderIndex(next))
			require.Equal(t, testCase.expected.len, reminderScheduler.Len())
			require.Equal(t, testCase.expected.hasNextDelay, hasDelay)
			require.Equal(t, testCase.expected.nextDelay, delay)
		})
	}
}

func TestSchedulerDueWithRealCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		advance  time.Duration
		seed     []database.ScheduledReminder
		expected dueExpectation
	}{
		{
			name:    "returns reminder after clock reaches due time",
			advance: 5 * time.Minute,
			seed:    []database.ScheduledReminder{{ID: 1, NoteID: 10, RemindAt: baseTime.Add(5 * time.Minute)}},
			expected: dueExpectation{
				due: []database.ScheduledReminder{{ID: 1, NoteID: 10, RemindAt: baseTime.Add(5 * time.Minute)}},
			},
		},
		{
			name:    "returns multiple due reminders",
			advance: 5 * time.Minute,
			seed: []database.ScheduledReminder{
				{ID: 2, NoteID: 20, RemindAt: baseTime.Add(2 * time.Minute)},
				{ID: 3, NoteID: 30, RemindAt: baseTime.Add(5 * time.Minute)},
			},
			expected: dueExpectation{
				due: []database.ScheduledReminder{
					{ID: 2, NoteID: 20, RemindAt: baseTime.Add(2 * time.Minute)},
					{ID: 3, NoteID: 30, RemindAt: baseTime.Add(5 * time.Minute)},
				},
			},
		},
		{
			name: "returns reminder that is already in the past",
			seed: []database.ScheduledReminder{
				{ID: 4, NoteID: 40, RemindAt: baseTime.Add(-time.Minute)},
			},
			expected: dueExpectation{
				due: []database.ScheduledReminder{{ID: 4, NoteID: 40, RemindAt: baseTime.Add(-time.Minute)}},
			},
		},
		{
			name: "returns reminder exactly at current time boundary",
			seed: []database.ScheduledReminder{{ID: 5, NoteID: 50, RemindAt: baseTime}},
			expected: dueExpectation{
				due: []database.ScheduledReminder{{ID: 5, NoteID: 50, RemindAt: baseTime}},
			},
		},
		{
			name:    "keeps reminder just before due boundary",
			advance: 5*time.Minute - time.Nanosecond,
			seed:    []database.ScheduledReminder{{ID: 6, NoteID: 60, RemindAt: baseTime.Add(5 * time.Minute)}},
			expected: dueExpectation{
				next:    database.ScheduledReminder{ID: 6, NoteID: 60, RemindAt: baseTime.Add(5 * time.Minute)},
				hasNext: true,
				len:     1,
			},
		},
		{
			name:    "keeps later reminder after due prefix is removed",
			advance: time.Minute,
			seed: []database.ScheduledReminder{
				{ID: 7, NoteID: 70, RemindAt: baseTime.Add(time.Minute)},
				{ID: 8, NoteID: 80, RemindAt: baseTime.Add(2 * time.Minute)},
			},
			expected: dueExpectation{
				due:     []database.ScheduledReminder{{ID: 7, NoteID: 70, RemindAt: baseTime.Add(time.Minute)}},
				next:    database.ScheduledReminder{ID: 8, NoteID: 80, RemindAt: baseTime.Add(2 * time.Minute)},
				hasNext: true,
				len:     1,
			},
		},
		{
			name: "returns empty due list on empty scheduler",
			expected: dueExpectation{
				due: []database.ScheduledReminder{},
			},
		},
		{
			name: "returns empty due list when all reminders are in future",
			seed: []database.ScheduledReminder{{ID: 10, NoteID: 100, RemindAt: baseTime.Add(time.Minute)}},
			expected: dueExpectation{
				next:    database.ScheduledReminder{ID: 10, NoteID: 100, RemindAt: baseTime.Add(time.Minute)},
				hasNext: true,
				len:     1,
			},
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

			require.ElementsMatch(t, testCase.expected.due, clearReminderIndexes(due))
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.next, clearReminderIndex(next))
			require.Equal(t, testCase.expected.len, reminderScheduler.Len())
		})
	}
}

func TestSchedulerScheduleErrors(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)
	reminder := database.ScheduledReminder{ID: 1, NoteID: 10, RemindAt: baseTime.Add(time.Minute)}

	testCases := []struct {
		name     string
		database func(t *testing.T) *mocks.MockDatabase
		expected scheduleExpectation
	}{
		{
			name: "wraps save reminder database error",
			database: func(t *testing.T) *mocks.MockDatabase {
				t.Helper()

				reminderDatabase := mocks.NewMockDatabase(gomock.NewController(t))
				reminderDatabase.EXPECT().
					SaveScheduledReminder(gomock.Any(), reminder).
					Return(errStorageUnavailable)

				return reminderDatabase
			},
			expected: scheduleExpectation{
				errText: "save scheduled reminder: storage unavailable",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testClock := clock.NewMock()
			testClock.Set(baseTime)
			reminderScheduler := scheduler.New(
				cache.NewReminderCache(time.Hour),
				testCase.database(t),
				scheduler.WithClock(testClock),
			)

			added, err := reminderScheduler.ScheduleReminder(context.Background(), reminder)

			require.EqualErrorf(t, err, testCase.expected.errText, "unexpected schedule error")
			require.False(t, added)
			require.Equal(t, 0, reminderScheduler.Len())
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
		seed      []database.ScheduledReminder
		checkID   int64
		expected  refreshExpectation
	}{
		{
			name:      "loads one reminder from database",
			now:       baseTime,
			window:    time.Hour,
			reminders: []database.ScheduledReminder{{ID: 1, NoteID: 10, RemindAt: baseTime.Add(time.Minute)}},
			checkID:   1,
			expected: refreshExpectation{
				loaded:  1,
				next:    database.ScheduledReminder{ID: 1, NoteID: 10, RemindAt: baseTime.Add(time.Minute)},
				hasNext: true,
				len:     1,
				exists:  true,
			},
		},
		{
			name:   "loads multiple reminders from database",
			now:    baseTime,
			window: time.Hour,
			reminders: []database.ScheduledReminder{
				{ID: 2, NoteID: 20, RemindAt: baseTime.Add(2 * time.Minute)},
				{ID: 3, NoteID: 30, RemindAt: baseTime.Add(time.Minute)},
			},
			checkID: 2,
			expected: refreshExpectation{
				loaded:  2,
				next:    database.ScheduledReminder{ID: 3, NoteID: 30, RemindAt: baseTime.Add(time.Minute)},
				hasNext: true,
				len:     2,
				exists:  true,
			},
		},
		{
			name:      "loads reminder with text payload",
			now:       baseTime,
			window:    time.Hour,
			reminders: []database.ScheduledReminder{{ID: 4, Text: "payload", RemindAt: baseTime}},
			checkID:   4,
			expected: refreshExpectation{
				loaded:  1,
				next:    database.ScheduledReminder{ID: 4, Text: "payload", RemindAt: baseTime},
				hasNext: true,
				len:     1,
				exists:  true,
			},
		},
		{
			name:      "loads reminder using non default window",
			now:       baseTime,
			window:    15 * time.Minute,
			reminders: []database.ScheduledReminder{{ID: 5, RemindAt: baseTime.Add(15 * time.Minute)}},
			checkID:   5,
			expected: refreshExpectation{
				loaded:  1,
				next:    database.ScheduledReminder{ID: 5, RemindAt: baseTime.Add(15 * time.Minute)},
				hasNext: true,
				len:     1,
				exists:  true,
			},
		},
		{
			name:     "loads no reminders on empty database result boundary",
			now:      baseTime,
			window:   time.Hour,
			checkID:  0,
			expected: refreshExpectation{},
		},
		{
			name:      "passes advanced clock time into database boundary",
			now:       baseTime.Add(time.Hour),
			window:    time.Hour,
			reminders: []database.ScheduledReminder{{ID: 6, RemindAt: baseTime.Add(time.Hour)}},
			checkID:   6,
			expected: refreshExpectation{
				loaded:  1,
				next:    database.ScheduledReminder{ID: 6, RemindAt: baseTime.Add(time.Hour)},
				hasNext: true,
				len:     1,
				exists:  true,
			},
		},
		{
			name:   "replaces stale cache contents with refreshed database window",
			now:    baseTime,
			window: time.Hour,
			seed: []database.ScheduledReminder{
				{ID: 7, NoteID: 70, RemindAt: baseTime.Add(time.Minute)},
			},
			reminders: []database.ScheduledReminder{
				{ID: 8, NoteID: 80, RemindAt: baseTime.Add(2 * time.Minute)},
			},
			checkID: 7,
			expected: refreshExpectation{
				loaded:  1,
				next:    database.ScheduledReminder{ID: 8, NoteID: 80, RemindAt: baseTime.Add(2 * time.Minute)},
				hasNext: true,
				len:     1,
				exists:  false,
			},
		},
		{
			name:   "wraps database error",
			now:    baseTime,
			window: time.Hour,
			err:    errStorageUnavailable,
			expected: refreshExpectation{
				errText: "load scheduled reminders: storage unavailable",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			reminderDatabase := mocks.NewMockDatabase(ctrl)
			reminderDatabase.EXPECT().
				ScheduledReminders(gomock.Any(), testCase.now, testCase.now.Add(testCase.window)).
				Return(testCase.reminders, testCase.err)

			testClock := clock.NewMock()
			testClock.Set(testCase.now)
			reminderScheduler := scheduler.New(
				cache.NewReminderCache(testCase.window),
				reminderDatabase,
				scheduler.WithClock(testClock),
			)

			for _, reminder := range testCase.seed {
				reminderScheduler.UpsertReminder(reminder)
			}

			loaded, err := reminderScheduler.Refresh(context.Background())
			next, hasNext := reminderScheduler.Next()

			if testCase.expected.errText != "" {
				require.EqualErrorf(t, err, testCase.expected.errText, "unexpected refresh error")
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, testCase.expected.loaded, loaded)
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.next, clearReminderIndex(next))
			require.Equal(t, testCase.expected.len, reminderScheduler.Len())
			require.Equal(t, testCase.expected.exists, reminderScheduler.Exists(testCase.checkID))
		})
	}
}

func TestSchedulerWithMockCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name   string
		assert func(
			t *testing.T,
			reminderScheduler *scheduler.Scheduler,
			reminderCache *mocks.MockCache,
			reminderDatabase *mocks.MockDatabase,
		)
	}{
		{
			name: "schedule reminder passes clock time and reminder to cache",
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				reminderDatabase *mocks.MockDatabase,
			) {
				t.Helper()

				reminder := database.ScheduledReminder{ID: 1, RemindAt: baseTime.Add(time.Minute)}
				reminderDatabase.EXPECT().SaveScheduledReminder(gomock.Any(), reminder).Return(nil)
				reminderCache.EXPECT().AddIfWithinWindow(baseTime, reminder).Return(true)

				added, err := reminderScheduler.ScheduleReminder(context.Background(), reminder)

				require.NoError(t, err)
				require.True(t, added)
			},
		},
		{
			name: "due passes current clock time to cache",
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				_ *mocks.MockDatabase,
			) {
				t.Helper()

				reminders := []database.ScheduledReminder{{ID: 2, RemindAt: baseTime}}
				reminderCache.EXPECT().PopDue(baseTime).Return(reminders)

				require.Equal(t, reminders, reminderScheduler.Due())
			},
		},
		{
			name: "cancel delegates reminder id to cache",
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				_ *mocks.MockDatabase,
			) {
				t.Helper()

				reminderCache.EXPECT().Remove(int64(3)).Return(true)

				require.True(t, reminderScheduler.Cancel(3))
			},
		},
		{
			name: "next delay returns false when cache has no next reminder",
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				_ *mocks.MockDatabase,
			) {
				t.Helper()

				reminderCache.EXPECT().Next().Return(database.ScheduledReminder{}, false)

				delay, ok := reminderScheduler.NextDelay()

				require.False(t, ok)
				require.Equal(t, time.Duration(0), delay)
			},
		},
		{
			name: "next delay returns future duration from cache reminder",
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				_ *mocks.MockDatabase,
			) {
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
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				_ *mocks.MockDatabase,
			) {
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
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				_ *mocks.MockDatabase,
			) {
				t.Helper()

				reminderCache.EXPECT().Window().Return(time.Hour)

				require.Equal(t, time.Hour, reminderScheduler.Window())
			},
		},
		{
			name: "len delegates to cache",
			assert: func(
				t *testing.T,
				reminderScheduler *scheduler.Scheduler,
				reminderCache *mocks.MockCache,
				_ *mocks.MockDatabase,
			) {
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
			reminderDatabase := mocks.NewMockDatabase(ctrl)
			testClock := clock.NewMock()
			testClock.Set(baseTime)
			reminderScheduler := scheduler.New(reminderCache, reminderDatabase, scheduler.WithClock(testClock))

			testCase.assert(t, reminderScheduler, reminderCache, reminderDatabase)
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
		expected      loadExpectation
	}{
		{
			name:          "loads one thousand reminders and drains all of them",
			reminderCount: 1_000,
			window:        24 * time.Hour,
			dueAt:         24 * time.Hour,
			expected: loadExpectation{
				due: 1_000,
			},
		},
		{
			name:          "loads ten thousand reminders and drains all of them",
			reminderCount: 10_000,
			window:        24 * time.Hour,
			dueAt:         24 * time.Hour,
			expected: loadExpectation{
				due: 10_000,
			},
		},
		{
			name:          "keeps half of ten thousand reminders outside due point",
			reminderCount: 10_000,
			window:        24 * time.Hour,
			dueAt:         12 * time.Hour,
			expected: loadExpectation{
				due:  5_001,
				rest: 4_999,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testClock := clock.NewMock()
			testClock.Set(baseTime)

			reminderDatabase := mocks.NewMockDatabase(gomock.NewController(t))
			reminderDatabase.EXPECT().
				SaveScheduledReminder(gomock.Any(), gomock.Any()).
				Return(nil).
				AnyTimes()

			reminderScheduler := scheduler.New(
				cache.NewReminderCache(testCase.window),
				reminderDatabase,
				scheduler.WithClock(testClock),
			)

			for index := range testCase.reminderCount {
				reminderID := int64(index + 1)
				reminderOffset := time.Duration(index%testCase.reminderCount) * testCase.window /
					time.Duration(testCase.reminderCount)

				added, err := reminderScheduler.ScheduleReminder(context.Background(), database.ScheduledReminder{
					ID:       reminderID,
					NoteID:   reminderID,
					RemindAt: baseTime.Add(reminderOffset),
				})

				require.NoError(t, err)
				require.True(t, added)
			}

			testClock.Add(testCase.dueAt)

			due := reminderScheduler.Due()

			require.Len(t, due, testCase.expected.due)
			require.Equal(t, testCase.expected.rest, reminderScheduler.Len())
		})
	}
}

func TestSchedulerConcurrentScheduleWithRealCache(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		parallel int
		expected concurrentExpectation
	}{
		{
			name:     "schedules eight hundred concurrent reminders",
			parallel: 800,
			expected: concurrentExpectation{
				len:   800,
				added: 800,
			},
		},
		{
			name:     "schedules nine hundred concurrent reminders",
			parallel: 900,
			expected: concurrentExpectation{
				len:   900,
				added: 900,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testClock := clock.NewMock()
			testClock.Set(baseTime)

			reminderDatabase := mocks.NewMockDatabase(gomock.NewController(t))
			reminderDatabase.EXPECT().
				SaveScheduledReminder(gomock.Any(), gomock.Any()).
				Return(nil).
				AnyTimes()

			reminderScheduler := scheduler.New(
				cache.NewReminderCache(24*time.Hour),
				reminderDatabase,
				scheduler.WithClock(testClock),
			)

			var (
				added     atomic.Int64
				waitGroup sync.WaitGroup
			)

			errs := make(chan error, testCase.parallel)

			for index := range testCase.parallel {
				waitGroup.Add(1)

				go func(index int) {
					defer waitGroup.Done()

					reminderID := int64(index + 1)
					reminder := database.ScheduledReminder{
						ID:       reminderID,
						NoteID:   reminderID,
						RemindAt: baseTime.Add(time.Duration(index) * time.Second),
					}

					wasAdded, err := reminderScheduler.ScheduleReminder(context.Background(), reminder)
					if err != nil {
						errs <- err

						return
					}

					if wasAdded {
						added.Add(1)
					}
				}(index)
			}

			waitGroup.Wait()
			close(errs)

			for err := range errs {
				require.NoError(t, err)
			}

			require.Equal(t, testCase.expected.added, added.Load())
			require.Equal(t, testCase.expected.len, reminderScheduler.Len())
		})
	}
}

func BenchmarkSchedulerScheduleAndDrainWithRealCache(b *testing.B) {
	baseTime := time.Date(2026, time.May, 22, 12, 0, 0, 0, time.UTC)

	for range b.N {
		testClock := clock.NewMock()
		testClock.Set(baseTime)

		reminderDatabase := mocks.NewMockDatabase(gomock.NewController(b))
		reminderDatabase.EXPECT().
			SaveScheduledReminder(gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		reminderScheduler := scheduler.New(
			cache.NewReminderCache(24*time.Hour),
			reminderDatabase,
			scheduler.WithClock(testClock),
		)

		for index := range 10_000 {
			reminderID := int64(index + 1)

			_, err := reminderScheduler.ScheduleReminder(context.Background(), database.ScheduledReminder{
				ID:       reminderID,
				NoteID:   reminderID,
				RemindAt: baseTime.Add(time.Duration(index) * time.Second),
			})
			require.NoError(b, err)
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

func expectedScheduledReminder(note database.Note, reminder database.Reminder) database.ScheduledReminder {
	return database.ScheduledReminder{
		ID:             reminder.ID,
		NoteID:         reminder.NoteID,
		TelegramChatID: note.TelegramChatID,
		Text:           note.Text,
		RemindAt:       reminder.RemindAt,
	}
}
