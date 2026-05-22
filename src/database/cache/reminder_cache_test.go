//nolint:cyclop,dupl,testpackage // linters was disabled because a long functions make tests better and more readable
package cache

import (
	"container/heap"
	"testing"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/stretchr/testify/require"
)

func TestNewScheduledReminder(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type input struct {
		note     database.Note
		reminder database.Reminder
	}

	type expected struct {
		reminder ScheduledReminder
	}

	testCases := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "copies all note and reminder fields into scheduled reminder",
			input: input{
				note:     database.Note{ID: 7, TelegramChatID: 101, Text: "Pay utilities"},
				reminder: database.Reminder{ID: 11, NoteID: 7, RemindAt: baseTime.Add(10 * time.Minute)},
			},
			expected: expected{reminder: fixtureReminder(11, 7, 101, "Pay utilities", baseTime.Add(10*time.Minute))},
		},
		{
			name: "preserves unicode and punctuation in reminder text",
			input: input{
				note:     database.Note{ID: 8, TelegramChatID: 102, Text: "Купить молоко, хлеб и сыр!"},
				reminder: database.Reminder{ID: 12, NoteID: 8, RemindAt: baseTime.Add(30 * time.Minute)},
			},
			expected: expected{reminder: fixtureReminder(12, 8, 102,
				"Купить молоко, хлеб и сыр!", baseTime.Add(30*time.Minute))},
		},
		{
			name: "keeps zero values when note and reminder identifiers are empty",
			input: input{
				note:     database.Note{},
				reminder: database.Reminder{RemindAt: baseTime},
			},
			expected: expected{reminder: fixtureReminder(0, 0, 0, "", baseTime)},
		},
		{
			name: "uses reminder note identifier instead of note primary key",
			input: input{
				note:     database.Note{ID: 90, TelegramChatID: 103, Text: "Detached reminder"},
				reminder: database.Reminder{ID: 13, NoteID: 91, RemindAt: baseTime.Add(45 * time.Minute)},
			},
			expected: expected{reminder: fixtureReminder(
				13, 91, 103, "Detached reminder", baseTime.Add(45*time.Minute))},
		},
		{
			name: "does not synthesize chat id when note has zero chat identifier",
			input: input{
				note:     database.Note{ID: 10, Text: "Missing chat"},
				reminder: database.Reminder{ID: 14, NoteID: 10, RemindAt: baseTime.Add(1 * time.Hour)},
			},
			expected: expected{reminder: fixtureReminder(
				14, 10, 0, "Missing chat", baseTime.Add(1*time.Hour))},
		},
		{
			name: "does not alter past reminder timestamps",
			input: input{
				note:     database.Note{ID: 11, TelegramChatID: 104, Text: "Past reminder"},
				reminder: database.Reminder{ID: 15, NoteID: 11, RemindAt: baseTime.Add(-15 * time.Minute)},
			},
			expected: expected{reminder: fixtureReminder(15, 11, 104, "Past reminder", baseTime.Add(-15*time.Minute))},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			actual := NewScheduledReminder(testCase.input.note, testCase.input.reminder)

			require.Equal(t, testCase.expected.reminder, actual)
		})
	}
}

func TestReminderHeapOperations(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type expected struct {
		lenAfterPush int
		lenAfterPop  int
		firstID      int64
		poppedIDs    []int64
		lessResult   bool
		indexes      map[int64]int
	}

	testCases := []struct {
		name         string
		popCount     int
		checkLess    *[2]int
		swapFirst    int
		swapSecond   int
		beforeScript func(t *testing.T, h *reminderHeap, insertedByID map[int64]*ScheduledReminder)
		expected     expected
	}{
		{
			name: "push assigns sequential indexes for multiple reminders",
			beforeScript: func(t *testing.T, rHeap *reminderHeap, insertedByID map[int64]*ScheduledReminder) {
				t.Helper()

				items := []ScheduledReminder{
					fixtureReminder(1, 0, 0, "", baseTime.Add(15*time.Minute)),
					fixtureReminder(2, 0, 0, "", baseTime.Add(30*time.Minute)),
					fixtureReminder(3, 0, 0, "", baseTime.Add(45*time.Minute)),
				}

				for _, item := range items {
					reminder := item
					heap.Push(rHeap, &reminder)
					insertedByID[reminder.ID] = &reminder
				}
			},
			expected: expected{
				lenAfterPush: 3,
				lenAfterPop:  3,
				firstID:      1,
				poppedIDs:    []int64{},
				indexes:      map[int64]int{1: 0, 2: 1, 3: 2},
			},
		},
		{
			name:      "less returns true when first reminder is scheduled earlier",
			checkLess: &[2]int{0, 1},
			beforeScript: func(t *testing.T, rHeap *reminderHeap, insertedByID map[int64]*ScheduledReminder) {
				t.Helper()

				items := []ScheduledReminder{
					fixtureReminder(4, 0, 0, "", baseTime.Add(5*time.Minute)),
					fixtureReminder(5, 0, 0, "", baseTime.Add(20*time.Minute)),
				}

				for _, item := range items {
					reminder := item
					heap.Push(rHeap, &reminder)
					insertedByID[reminder.ID] = &reminder
				}
			},
			expected: expected{
				lenAfterPush: 2,
				lenAfterPop:  2,
				firstID:      4,
				poppedIDs:    []int64{},
				lessResult:   true,
				indexes:      map[int64]int{4: 0, 5: 1},
			},
		},
		{
			name:       "swap exchanges reminders and refreshes both indexes",
			swapFirst:  0,
			swapSecond: 1,
			beforeScript: func(t *testing.T, rHeap *reminderHeap, insertedByID map[int64]*ScheduledReminder) {
				t.Helper()

				items := []ScheduledReminder{
					fixtureReminder(6, 0, 0, "", baseTime.Add(10*time.Minute)),
					fixtureReminder(7, 0, 0, "", baseTime.Add(25*time.Minute)),
				}

				for _, item := range items {
					reminder := item
					heap.Push(rHeap, &reminder)
					insertedByID[reminder.ID] = &reminder
				}
			},
			expected: expected{
				lenAfterPush: 2,
				lenAfterPop:  2,
				firstID:      7,
				poppedIDs:    []int64{},
				indexes:      map[int64]int{6: 1, 7: 0},
			},
		},
		{
			name:     "pop removes reminders from the tail and marks their index as removed",
			popCount: 2,
			beforeScript: func(t *testing.T, rHeap *reminderHeap, insertedByID map[int64]*ScheduledReminder) {
				t.Helper()

				items := []ScheduledReminder{
					fixtureReminder(8, 0, 0, "", baseTime.Add(10*time.Minute)),
					fixtureReminder(9, 0, 0, "", baseTime.Add(20*time.Minute)),
					fixtureReminder(10, 0, 0, "", baseTime.Add(30*time.Minute)),
				}

				for _, item := range items {
					reminder := item
					heap.Push(rHeap, &reminder)
					insertedByID[reminder.ID] = &reminder
				}
			},
			expected: expected{
				lenAfterPush: 3,
				lenAfterPop:  1,
				firstID:      8,
				poppedIDs:    []int64{10, 9},
				indexes:      map[int64]int{8: 0},
			},
		},
		{
			name:      "less returns false when reminders have identical timestamps",
			checkLess: &[2]int{0, 1},
			beforeScript: func(t *testing.T, rHeap *reminderHeap, insertedByID map[int64]*ScheduledReminder) {
				t.Helper()

				items := []ScheduledReminder{
					fixtureReminder(11, 0, 0, "", baseTime.Add(40*time.Minute)),
					fixtureReminder(12, 0, 0, "", baseTime.Add(40*time.Minute)),
				}

				for _, item := range items {
					reminder := item
					heap.Push(rHeap, &reminder)
					insertedByID[reminder.ID] = &reminder
				}
			},
			expected: expected{
				lenAfterPush: 2,
				lenAfterPop:  2,
				firstID:      11,
				poppedIDs:    []int64{},
				lessResult:   false,
				indexes:      map[int64]int{11: 0, 12: 1},
			},
		},
		{
			name:     "single pop leaves heap empty",
			popCount: 1,
			beforeScript: func(t *testing.T, h *reminderHeap, insertedByID map[int64]*ScheduledReminder) {
				t.Helper()

				item := fixtureReminder(13, 0, 0, "", baseTime.Add(50*time.Minute))
				heap.Push(h, &item)
				insertedByID[item.ID] = &item
			},
			expected: expected{
				lenAfterPush: 1,
				lenAfterPop:  0,
				poppedIDs:    []int64{13},
				indexes:      map[int64]int{},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rHeap := make(reminderHeap, 0)
			insertedByID := make(map[int64]*ScheduledReminder)

			if testCase.beforeScript != nil {
				testCase.beforeScript(t, &rHeap, insertedByID)
			}

			require.Equal(t, testCase.expected.lenAfterPush, rHeap.Len())

			if testCase.checkLess != nil {
				require.Equal(t, testCase.expected.lessResult, rHeap.Less(testCase.checkLess[0], testCase.checkLess[1]))
			}

			if testCase.swapFirst != testCase.swapSecond || testCase.swapFirst != 0 {
				rHeap.Swap(testCase.swapFirst, testCase.swapSecond)
			}

			if testCase.expected.lenAfterPop > 0 {
				require.Equal(t, testCase.expected.firstID, rHeap[0].ID)
			}

			poppedIDs := make([]int64, 0, testCase.popCount)
			for range testCase.popCount {
				item, ok := rHeap.Pop().(*ScheduledReminder)
				require.True(t, ok)

				poppedIDs = append(poppedIDs, item.ID)
				require.Equal(t, -1, item.Index)
			}

			require.Equal(t, testCase.expected.poppedIDs, poppedIDs)
			require.Equal(t, testCase.expected.lenAfterPop, rHeap.Len())

			actualIndexes := make(map[int64]int, len(insertedByID))
			for id := range testCase.expected.indexes {
				require.Contains(t, insertedByID, id)
				actualIndexes[id] = insertedByID[id].Index
			}

			require.Equal(t, testCase.expected.indexes, actualIndexes)
		})
	}
}

func TestReminderCacheAddIfWithinWindow(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type input struct {
		now       time.Time
		reminders []ScheduledReminder
	}

	type expected struct {
		addedResults []bool
		nextReminder ScheduledReminder
		hasNext      bool
		existingIDs  []int64
		missingIDs   []int64
		cacheLen     int
		window       time.Duration
	}

	testCases := []struct {
		name        string
		cacheWindow time.Duration
		input       input
		expected    expected
	}{
		{
			name:        "add within window inserts reminders and keeps earliest next reminder",
			cacheWindow: 1 * time.Hour,
			input: input{
				now: baseTime,
				reminders: []ScheduledReminder{
					fixtureReminder(1, 101, 1001, "first", baseTime.Add(30*time.Minute)),
					fixtureReminder(2, 102, 1002, "second", baseTime.Add(10*time.Minute)),
				},
			},
			expected: expected{
				addedResults: []bool{true, true},
				nextReminder: fixtureReminder(2, 102, 1002, "second", baseTime.Add(10*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{1, 2},
				missingIDs:   []int64{3, 4},
				cacheLen:     2,
				window:       1 * time.Hour,
			},
		},
		{
			name:        "add on exact window boundary is accepted",
			cacheWindow: 45 * time.Minute,
			input: input{
				now:       baseTime,
				reminders: []ScheduledReminder{fixtureReminder(5, 105, 1005, "boundary", baseTime.Add(45*time.Minute))},
			},
			expected: expected{
				addedResults: []bool{true},
				nextReminder: fixtureReminder(5, 105, 1005, "boundary", baseTime.Add(45*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{5},
				missingIDs:   []int64{1, 6},
				cacheLen:     1,
				window:       45 * time.Minute,
			},
		},
		{
			name:        "later add does not replace earlier next reminder",
			cacheWindow: 2 * time.Hour,
			input: input{
				now: baseTime,
				reminders: []ScheduledReminder{
					fixtureReminder(9, 109, 1009, "earlier", baseTime.Add(25*time.Minute)),
					fixtureReminder(10, 110, 1010, "later", baseTime.Add(35*time.Minute)),
				},
			},
			expected: expected{
				addedResults: []bool{true, true},
				nextReminder: fixtureReminder(9, 109, 1009, "earlier", baseTime.Add(25*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{9, 10},
				missingIDs:   []int64{11, 12},
				cacheLen:     2,
				window:       2 * time.Hour,
			},
		},
		{
			name:        "past due reminder is still accepted because it is inside the window",
			cacheWindow: 15 * time.Minute,
			input: input{
				now: baseTime,
				reminders: []ScheduledReminder{fixtureReminder(
					16, 116, 1016, "past due", baseTime.Add(-1*time.Minute))},
			},
			expected: expected{
				addedResults: []bool{true},
				nextReminder: fixtureReminder(16, 116, 1016, "past due", baseTime.Add(-1*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{16},
				missingIDs:   []int64{17, 18},
				cacheLen:     1,
				window:       15 * time.Minute,
			},
		},
		{
			name:        "add after window rejects reminder and leaves cache empty",
			cacheWindow: 20 * time.Minute,
			input: input{
				now:       baseTime,
				reminders: []ScheduledReminder{fixtureReminder(8, 108, 1008, "too far", baseTime.Add(21*time.Minute))},
			},
			expected: expected{
				addedResults: []bool{false},
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				existingIDs:  []int64{},
				missingIDs:   []int64{8, 10},
				cacheLen:     0,
				window:       20 * time.Minute,
			},
		},
		{
			name:        "mix of accepted and rejected reminders keeps only reminders inside window",
			cacheWindow: 30 * time.Minute,
			input: input{
				now: baseTime,
				reminders: []ScheduledReminder{
					fixtureReminder(19, 119, 1019, "inside", baseTime.Add(5*time.Minute)),
					fixtureReminder(20, 120, 1020, "outside", baseTime.Add(31*time.Minute)),
				},
			},
			expected: expected{
				addedResults: []bool{true, false},
				nextReminder: fixtureReminder(19, 119, 1019, "inside", baseTime.Add(5*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{19},
				missingIDs:   []int64{20, 21},
				cacheLen:     1,
				window:       30 * time.Minute,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cache := NewReminderCache(testCase.cacheWindow)
			addedResults := make([]bool, 0, len(testCase.input.reminders))

			for _, reminder := range testCase.input.reminders {
				addedResults = append(addedResults, cache.AddIfWithinWindow(testCase.input.now, reminder))
			}

			nextReminder, hasNext := cache.Next()

			require.Equal(t, testCase.expected.addedResults, addedResults)
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.nextReminder, nextReminder)

			for _, existingID := range testCase.expected.existingIDs {
				require.True(t, cache.Exists(existingID))
			}

			for _, missingID := range testCase.expected.missingIDs {
				require.False(t, cache.Exists(missingID))
			}

			require.Equal(t, testCase.expected.cacheLen, cache.Len())
			require.Equal(t, testCase.expected.window, cache.Window())
		})
	}
}

func TestReminderCacheUpsert(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type input struct {
		reminders []ScheduledReminder
	}

	type expected struct {
		nextReminder ScheduledReminder
		hasNext      bool
		existingIDs  []int64
		missingIDs   []int64
		cacheLen     int
	}

	testCases := []struct {
		name        string
		cacheWindow time.Duration
		input       input
		expected    expected
	}{
		{
			name:        "upsert inserts multiple new reminders and keeps earliest next reminder",
			cacheWindow: 2 * time.Hour,
			input: input{
				reminders: []ScheduledReminder{
					fixtureReminder(3, 103, 1003, "draft", baseTime.Add(90*time.Minute)),
					fixtureReminder(4, 104, 1004, "stable", baseTime.Add(40*time.Minute)),
				},
			},
			expected: expected{
				nextReminder: fixtureReminder(4, 104, 1004, "stable", baseTime.Add(40*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{3, 4},
				missingIDs:   []int64{2, 5},
				cacheLen:     2,
			},
		},
		{
			name:        "upsert updates existing reminder payload and reorders heap when time becomes earlier",
			cacheWindow: 2 * time.Hour,
			input: input{
				reminders: []ScheduledReminder{
					fixtureReminder(3, 103, 1003, "draft", baseTime.Add(90*time.Minute)),
					fixtureReminder(4, 104, 1004, "stable", baseTime.Add(40*time.Minute)),
					fixtureReminder(3, 303, 3003, "updated", baseTime.Add(5*time.Minute)),
				},
			},
			expected: expected{
				nextReminder: fixtureReminder(3, 303, 3003, "updated", baseTime.Add(5*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{3, 4},
				missingIDs:   []int64{2, 5},
				cacheLen:     2,
			},
		},
		{
			name:        "upsert inserts reminders even when their time is already in the past",
			cacheWindow: 15 * time.Minute,
			input: input{
				reminders: []ScheduledReminder{
					fixtureReminder(6, 106, 1006, "past due", baseTime.Add(-1*time.Minute)),
					fixtureReminder(7, 107, 1007, "future", baseTime.Add(14*time.Minute)),
				},
			},
			expected: expected{
				nextReminder: fixtureReminder(6, 106, 1006, "past due", baseTime.Add(-1*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{6, 7},
				missingIDs:   []int64{8, 9},
				cacheLen:     2,
			},
		},
		{
			name:        "upsert replaces text and chat id for existing reminder without duplicating cache entries",
			cacheWindow: 1 * time.Hour,
			input: input{
				reminders: []ScheduledReminder{
					fixtureReminder(22, 122, 1022, "old text", baseTime.Add(20*time.Minute)),
					fixtureReminder(22, 222, 2022, "new text", baseTime.Add(20*time.Minute)),
				},
			},
			expected: expected{
				nextReminder: fixtureReminder(22, 222, 2022, "new text", baseTime.Add(20*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{22},
				missingIDs:   []int64{23, 24},
				cacheLen:     1,
			},
		},
		{
			name:        "upsert with same id keeps only one cache entry when reminder is overwritten later",
			cacheWindow: 1 * time.Hour,
			input: input{
				reminders: []ScheduledReminder{
					fixtureReminder(25, 125, 1025, "initial", baseTime.Add(50*time.Minute)),
					fixtureReminder(25, 125, 1025, "earlier", baseTime.Add(10*time.Minute)),
				},
			},
			expected: expected{
				nextReminder: fixtureReminder(25, 125, 1025, "earlier", baseTime.Add(10*time.Minute)),
				hasNext:      true,
				existingIDs:  []int64{25},
				missingIDs:   []int64{26, 27},
				cacheLen:     1,
			},
		},
		{
			name:        "upsert on empty cache with zero value reminder still stores entry",
			cacheWindow: 1 * time.Hour,
			input: input{
				reminders: []ScheduledReminder{
					fixtureReminder(0, 0, 0, "", time.Time{}),
				},
			},
			expected: expected{
				nextReminder: fixtureReminder(0, 0, 0, "", time.Time{}),
				hasNext:      true,
				existingIDs:  []int64{0},
				missingIDs:   []int64{1, 2},
				cacheLen:     1,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cache := NewReminderCache(testCase.cacheWindow)
			for _, reminder := range testCase.input.reminders {
				cache.Upsert(reminder)
			}

			nextReminder, hasNext := cache.Next()

			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.nextReminder, nextReminder)

			for _, existingID := range testCase.expected.existingIDs {
				require.True(t, cache.Exists(existingID))
			}

			for _, missingID := range testCase.expected.missingIDs {
				require.False(t, cache.Exists(missingID))
			}

			require.Equal(t, testCase.expected.cacheLen, cache.Len())
		})
	}
}

func TestReminderCachePopDue(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type expected struct {
		poppedIDs    []int64
		nextReminder ScheduledReminder
		hasNext      bool
		cacheLen     int
		existingIDs  []int64
		missingIDs   []int64
	}

	testCases := []struct {
		name         string
		popAt        time.Time
		beforeScript func(t *testing.T, cache *ReminderCache)
		expected     expected
	}{
		{
			name:  "pop due returns all reminders scheduled before current time in chronological order",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(1, 201, 2001, "earliest", baseTime.Add(-10*time.Minute)))
				cache.Upsert(fixtureReminder(2, 202, 2002, "middle", baseTime.Add(-5*time.Minute)))
				cache.Upsert(fixtureReminder(3, 203, 2003, "future", baseTime.Add(10*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{1, 2},
				nextReminder: fixtureReminder(3, 203, 2003, "future", baseTime.Add(10*time.Minute)),
				hasNext:      true,
				cacheLen:     1,
				existingIDs:  []int64{3},
				missingIDs:   []int64{1, 2},
			},
		},
		{
			name:  "pop due includes reminders that match current time exactly",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(4, 204, 2004, "exact", baseTime))
				cache.Upsert(fixtureReminder(5, 205, 2005, "later", baseTime.Add(1*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{4},
				nextReminder: fixtureReminder(5, 205, 2005, "later", baseTime.Add(1*time.Minute)),
				hasNext:      true,
				cacheLen:     1,
				existingIDs:  []int64{5},
				missingIDs:   []int64{4, 6},
			},
		},
		{
			name:  "pop due drains cache completely when all reminders are due",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(9, 209, 2009, "keep one", baseTime.Add(-3*time.Minute)))
				cache.Upsert(fixtureReminder(10, 210, 2010, "keep two", baseTime.Add(-2*time.Minute)))
				cache.Upsert(fixtureReminder(11, 211, 2011, "keep three", baseTime.Add(-1*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{9, 10, 11},
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{9, 10, 11},
			},
		},
		{
			name:  "pop due returns empty slice when no reminders are due yet",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(14, 214, 2014, "later one", baseTime.Add(6*time.Minute)))
				cache.Upsert(fixtureReminder(15, 215, 2015, "later two", baseTime.Add(9*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{},
				nextReminder: fixtureReminder(14, 214, 2014, "later one", baseTime.Add(6*time.Minute)),
				hasNext:      true,
				cacheLen:     2,
				existingIDs:  []int64{14, 15},
				missingIDs:   []int64{16, 17},
			},
		},
		{
			name:  "pop due on empty cache returns empty slice",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
			},
			expected: expected{
				poppedIDs:    []int64{},
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{18, 19},
			},
		},
		{
			name:  "pop due removes only the due prefix and keeps later reminders in cache",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(28, 228, 2028, "due first", baseTime.Add(-2*time.Minute)))
				cache.Upsert(fixtureReminder(29, 229, 2029, "due second", baseTime.Add(-1*time.Minute)))
				cache.Upsert(fixtureReminder(30, 230, 2030, "future", baseTime.Add(20*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{28, 29},
				nextReminder: fixtureReminder(30, 230, 2030, "future", baseTime.Add(20*time.Minute)),
				hasNext:      true,
				cacheLen:     1,
				existingIDs:  []int64{30},
				missingIDs:   []int64{28, 29},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cache := NewReminderCache(1 * time.Hour)
			if testCase.beforeScript != nil {
				testCase.beforeScript(t, cache)
			}

			popped := cache.PopDue(testCase.popAt)

			poppedIDs := make([]int64, 0, len(popped))
			for _, reminder := range popped {
				poppedIDs = append(poppedIDs, reminder.ID)
			}

			nextReminder, hasNext := cache.Next()

			require.Equal(t, testCase.expected.poppedIDs, poppedIDs)
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.nextReminder, nextReminder)
			require.Equal(t, testCase.expected.cacheLen, cache.Len())

			for _, existingID := range testCase.expected.existingIDs {
				require.True(t, cache.Exists(existingID))
			}

			for _, missingID := range testCase.expected.missingIDs {
				require.False(t, cache.Exists(missingID))
			}
		})
	}
}

func TestReminderCachePopDueWithIdenticalTime(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type expected struct {
		poppedIDs    []int64
		nextReminder ScheduledReminder
		hasNext      bool
		cacheLen     int
		existingIDs  []int64
		missingIDs   []int64
	}

	testCases := []struct {
		name         string
		popAt        time.Time
		beforeScript func(t *testing.T, cache *ReminderCache)
		expected     expected
	}{
		{
			name:  "pop due returns all reminders that share the same due time",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(40, 240, 2040, "same time first", baseTime))
				cache.Upsert(fixtureReminder(41, 241, 2041, "same time second", baseTime))
				cache.Upsert(fixtureReminder(42, 242, 2042, "future", baseTime.Add(1*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{40, 41},
				nextReminder: fixtureReminder(42, 242, 2042, "future", baseTime.Add(1*time.Minute)),
				hasNext:      true,
				cacheLen:     1,
				existingIDs:  []int64{42},
				missingIDs:   []int64{40, 41},
			},
		},
		{
			name:  "pop due returns all identical-time reminders when cache contains only due entries",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(43, 243, 2043, "first", baseTime))
				cache.Upsert(fixtureReminder(44, 244, 2044, "second", baseTime))
				cache.Upsert(fixtureReminder(45, 245, 2045, "third", baseTime))
			},
			expected: expected{
				poppedIDs:    []int64{43, 44, 45},
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{43, 44, 45},
			},
		},
		{
			name:  "pop due does not skip earlier inserted reminder with identical due time",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(46, 246, 2046, "older", baseTime))
				cache.Upsert(fixtureReminder(47, 247, 2047, "newer", baseTime))
				cache.Upsert(fixtureReminder(48, 248, 2048, "later", baseTime.Add(2*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{46, 47},
				nextReminder: fixtureReminder(48, 248, 2048, "later", baseTime.Add(2*time.Minute)),
				hasNext:      true,
				cacheLen:     1,
				existingIDs:  []int64{48},
				missingIDs:   []int64{46, 47},
			},
		},
		{
			name:  "pop due with identical future timestamps leaves cache untouched before due time",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(49, 249, 2049, "future first", baseTime.Add(3*time.Minute)))
				cache.Upsert(fixtureReminder(50, 250, 2050, "future second", baseTime.Add(3*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{},
				nextReminder: fixtureReminder(49, 249, 2049, "future first", baseTime.Add(3*time.Minute)),
				hasNext:      true,
				cacheLen:     2,
				existingIDs:  []int64{49, 50},
				missingIDs:   []int64{51, 52},
			},
		},
		{
			name:  "pop due with identical time on empty cache returns no reminders",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
			},
			expected: expected{
				poppedIDs:    []int64{},
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{53, 54},
			},
		},
		{
			name:  "pop due keeps later distinct reminder after removing identical due timestamps",
			popAt: baseTime,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(55, 255, 2055, "same time one", baseTime))
				cache.Upsert(fixtureReminder(56, 256, 2056, "same time two", baseTime))
				cache.Upsert(fixtureReminder(57, 257, 2057, "later distinct", baseTime.Add(10*time.Minute)))
			},
			expected: expected{
				poppedIDs:    []int64{55, 56},
				nextReminder: fixtureReminder(57, 257, 2057, "later distinct", baseTime.Add(10*time.Minute)),
				hasNext:      true,
				cacheLen:     1,
				existingIDs:  []int64{57},
				missingIDs:   []int64{55, 56},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cache := NewReminderCache(1 * time.Hour)
			if testCase.beforeScript != nil {
				testCase.beforeScript(t, cache)
			}

			popped := cache.PopDue(testCase.popAt)

			poppedIDs := make([]int64, 0, len(popped))
			for _, reminder := range popped {
				poppedIDs = append(poppedIDs, reminder.ID)
			}

			nextReminder, hasNext := cache.Next()

			require.ElementsMatch(t, testCase.expected.poppedIDs, poppedIDs)
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.nextReminder, nextReminder)
			require.Equal(t, testCase.expected.cacheLen, cache.Len())

			for _, existingID := range testCase.expected.existingIDs {
				require.True(t, cache.Exists(existingID))
			}

			for _, missingID := range testCase.expected.missingIDs {
				require.False(t, cache.Exists(missingID))
			}
		})
	}
}

func TestReminderCacheRemove(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type expected struct {
		removed      bool
		nextReminder ScheduledReminder
		hasNext      bool
		cacheLen     int
		existingIDs  []int64
		missingIDs   []int64
	}

	testCases := []struct {
		name         string
		removeID     int64
		beforeScript func(t *testing.T, cache *ReminderCache)
		expected     expected
	}{
		{
			name:     "remove deletes existing reminder and keeps the next earliest item accessible",
			removeID: 6,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(6, 206, 2006, "remove me", baseTime.Add(2*time.Minute)))
				cache.Upsert(fixtureReminder(7, 207, 2007, "stay first", baseTime.Add(1*time.Minute)))
				cache.Upsert(fixtureReminder(8, 208, 2008, "stay later", baseTime.Add(5*time.Minute)))
			},
			expected: expected{
				removed:      true,
				nextReminder: fixtureReminder(7, 207, 2007, "stay first", baseTime.Add(1*time.Minute)),
				hasNext:      true,
				cacheLen:     2,
				existingIDs:  []int64{7, 8},
				missingIDs:   []int64{6, 9},
			},
		},
		{
			name:     "remove earliest reminder promotes the next reminder",
			removeID: 31,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(31, 231, 2031, "earliest", baseTime.Add(1*time.Minute)))
				cache.Upsert(fixtureReminder(32, 232, 2032, "next", baseTime.Add(2*time.Minute)))
			},
			expected: expected{
				removed:      true,
				nextReminder: fixtureReminder(32, 232, 2032, "next", baseTime.Add(2*time.Minute)),
				hasNext:      true,
				cacheLen:     1,
				existingIDs:  []int64{32},
				missingIDs:   []int64{31, 33},
			},
		},
		{
			name:     "remove last remaining reminder leaves cache empty",
			removeID: 34,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(34, 234, 2034, "single", baseTime.Add(3*time.Minute)))
			},
			expected: expected{
				removed:      true,
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{34, 35},
			},
		},
		{
			name:     "remove middle reminder keeps both outer reminders in cache",
			removeID: 36,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(35, 235, 2035, "first", baseTime.Add(1*time.Minute)))
				cache.Upsert(fixtureReminder(36, 236, 2036, "middle", baseTime.Add(2*time.Minute)))
				cache.Upsert(fixtureReminder(37, 237, 2037, "last", baseTime.Add(3*time.Minute)))
			},
			expected: expected{
				removed:      true,
				nextReminder: fixtureReminder(35, 235, 2035, "first", baseTime.Add(1*time.Minute)),
				hasNext:      true,
				cacheLen:     2,
				existingIDs:  []int64{35, 37},
				missingIDs:   []int64{36, 38},
			},
		},
		{
			name:     "remove returns false when reminder identifier does not exist",
			removeID: 99,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
				cache.Upsert(fixtureReminder(12, 212, 2012, "keep first", baseTime.Add(4*time.Minute)))
				cache.Upsert(fixtureReminder(13, 213, 2013, "keep second", baseTime.Add(8*time.Minute)))
			},
			expected: expected{
				removed:      false,
				nextReminder: fixtureReminder(12, 212, 2012, "keep first", baseTime.Add(4*time.Minute)),
				hasNext:      true,
				cacheLen:     2,
				existingIDs:  []int64{12, 13},
				missingIDs:   []int64{99, 14},
			},
		},
		{
			name:     "remove on empty cache returns false",
			removeID: 1,
			beforeScript: func(t *testing.T, cache *ReminderCache) {
				t.Helper()
			},
			expected: expected{
				removed:      false,
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{1, 3},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cache := NewReminderCache(1 * time.Hour)
			if testCase.beforeScript != nil {
				testCase.beforeScript(t, cache)
			}

			removed := cache.Remove(testCase.removeID)
			nextReminder, hasNext := cache.Next()

			require.Equal(t, testCase.expected.removed, removed)
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.nextReminder, nextReminder)
			require.Equal(t, testCase.expected.cacheLen, cache.Len())

			for _, existingID := range testCase.expected.existingIDs {
				require.True(t, cache.Exists(existingID))
			}

			for _, missingID := range testCase.expected.missingIDs {
				require.False(t, cache.Exists(missingID))
			}
		})
	}
}

func TestReminderCacheEmptyState(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, time.May, 21, 12, 0, 0, 0, time.UTC)

	type expected struct {
		nextReminder ScheduledReminder
		hasNext      bool
		removed      []bool
		poppedIDs    []int64
		cacheLen     int
		existingIDs  []int64
		missingIDs   []int64
	}

	testCases := []struct {
		name         string
		beforeScript func(t *testing.T, cache *ReminderCache)
		runScript    func(t *testing.T, cache *ReminderCache) ([]bool, []int64)
		expected     expected
	}{
		{
			name: "next on empty cache returns no reminder",
			runScript: func(t *testing.T, cache *ReminderCache) ([]bool, []int64) {
				t.Helper()

				return []bool{}, []int64{}
			},
			expected: expected{
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				removed:      []bool{},
				poppedIDs:    []int64{},
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{1, 2},
			},
		},
		{
			name: "pop due on empty cache returns empty slice",
			runScript: func(t *testing.T, cache *ReminderCache) ([]bool, []int64) {
				t.Helper()

				popped := cache.PopDue(baseTime)

				poppedIDs := make([]int64, 0, len(popped))
				for _, reminder := range popped {
					poppedIDs = append(poppedIDs, reminder.ID)
				}

				return []bool{}, poppedIDs
			},
			expected: expected{
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				removed:      []bool{},
				poppedIDs:    []int64{},
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{4, 5},
			},
		},
		{
			name: "window is retained even when cache has no reminders",
			runScript: func(t *testing.T, cache *ReminderCache) ([]bool, []int64) {
				t.Helper()

				return []bool{}, []int64{}
			},
			expected: expected{
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				removed:      []bool{},
				poppedIDs:    []int64{},
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{6, 7},
			},
		},
		{
			name: "empty cache never reports nonexistent ids as present",
			runScript: func(t *testing.T, cache *ReminderCache) ([]bool, []int64) {
				t.Helper()

				return []bool{}, []int64{}
			},
			expected: expected{
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				removed:      []bool{},
				poppedIDs:    []int64{},
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{8, 9},
			},
		},
		{
			name: "remove on empty cache returns false",
			runScript: func(t *testing.T, cache *ReminderCache) ([]bool, []int64) {
				t.Helper()

				return []bool{cache.Remove(1)}, []int64{}
			},
			expected: expected{
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				removed:      []bool{false},
				poppedIDs:    []int64{},
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{1, 3},
			},
		},
		{
			name: "multiple empty operations preserve empty state",
			runScript: func(t *testing.T, cache *ReminderCache) ([]bool, []int64) {
				t.Helper()

				popped := cache.PopDue(baseTime)

				poppedIDs := make([]int64, 0, len(popped))
				for _, reminder := range popped {
					poppedIDs = append(poppedIDs, reminder.ID)
				}

				return []bool{cache.Remove(10)}, poppedIDs
			},
			expected: expected{
				nextReminder: ScheduledReminder{},
				hasNext:      false,
				removed:      []bool{false},
				poppedIDs:    []int64{},
				cacheLen:     0,
				existingIDs:  []int64{},
				missingIDs:   []int64{10, 11},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cache := NewReminderCache(30 * time.Minute)
			if testCase.beforeScript != nil {
				testCase.beforeScript(t, cache)
			}

			removed, poppedIDs := testCase.runScript(t, cache)
			nextReminder, hasNext := cache.Next()

			require.Equal(t, testCase.expected.nextReminder, nextReminder)
			require.Equal(t, testCase.expected.hasNext, hasNext)
			require.Equal(t, testCase.expected.removed, removed)
			require.Equal(t, testCase.expected.poppedIDs, poppedIDs)
			require.Equal(t, testCase.expected.cacheLen, cache.Len())
			require.Equal(t, 30*time.Minute, cache.Window())

			for _, existingID := range testCase.expected.existingIDs {
				require.True(t, cache.Exists(existingID))
			}

			for _, missingID := range testCase.expected.missingIDs {
				require.False(t, cache.Exists(missingID))
			}
		})
	}
}

func fixtureReminder(id, noteID, chatID int64, text string, remindAt time.Time) ScheduledReminder {
	return ScheduledReminder{
		ID:             id,
		NoteID:         noteID,
		TelegramChatID: chatID,
		Text:           text,
		RemindAt:       remindAt,
	}
}
