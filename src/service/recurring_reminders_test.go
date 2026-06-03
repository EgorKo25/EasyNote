package service_test

import (
	"testing"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/stretchr/testify/require"
)

func TestCalculateNextReminder(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 6, 5, 9, 30, 0, 0, time.Local)
	intervalOne := 1
	intervalTwo := 2
	intervalSeven := 7
	invalidInterval := 0
	workdaysMask := database.WeekdaysMaskWorkdays
	tuesdayThursdayMask := 2 | 8
	invalidMask := 0
	nextCustomID := int64(10)
	expectedUnsupportedRule := "unsupported reminder rule type: unsupported"
	expectedInvalidInterval := "invalid interval reminder rule"
	expectedInvalidMask := "invalid weekdays reminder rule"

	tests := []struct {
		name     string
		input    database.ReminderCompletionInput
		expected struct {
			next        database.ReminderCompletionNext
			expectedErr string
		}
	}{
		{
			name: "daily interval creates next reminder from previous remind at plus one day",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 1, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:         database.ReminderRuleTypeInterval,
					IntervalDays: &intervalOne,
					Status:       database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{next: database.ReminderCompletionNext{RemindAt: ptrTime(base.AddDate(0, 0, 1))}},
		},
		{
			name: "two day interval creates next reminder from previous remind at plus two days",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 2, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:         database.ReminderRuleTypeInterval,
					IntervalDays: &intervalTwo,
					Status:       database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{next: database.ReminderCompletionNext{RemindAt: ptrTime(base.AddDate(0, 0, 2))}},
		},
		{
			name: "weekly interval creates next reminder from previous remind at plus seven days",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 3, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:         database.ReminderRuleTypeInterval,
					IntervalDays: &intervalSeven,
					Status:       database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{next: database.ReminderCompletionNext{RemindAt: ptrTime(base.AddDate(0, 0, 7))}},
		},
		{
			name: "weekdays rule creates monday reminder after friday",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 4, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:         database.ReminderRuleTypeWeekdays,
					WeekdaysMask: &workdaysMask,
					Status:       database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{next: database.ReminderCompletionNext{RemindAt: ptrTime(base.AddDate(0, 0, 3))}},
		},
		{
			name: "custom weekdays rule creates next reminder using selected weekday mask",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{
					ID:       10,
					RemindAt: time.Date(2026, 6, 2, 9, 30, 0, 0, time.Local),
				},
				Rule: &database.ReminderRule{
					Type:         database.ReminderRuleTypeWeekdays,
					WeekdaysMask: &tuesdayThursdayMask,
					Status:       database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{
				next: database.ReminderCompletionNext{
					RemindAt: ptrTime(time.Date(2026, 6, 4, 9, 30, 0, 0, time.Local)),
				},
			},
		},
		{
			name: "custom dates creates next reminder for first date without generated reminder",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 5, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:   database.ReminderRuleTypeCustomDates,
					Status: database.ReminderRuleStatusActive,
				},
				CustomDates: []database.CustomReminderDate{
					{RemindDate: base.AddDate(0, 0, 2), GeneratedReminderID: &nextCustomID},
					{RemindDate: base.AddDate(0, 0, 4)},
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{next: database.ReminderCompletionNext{RemindAt: ptrTime(base.AddDate(0, 0, 4))}},
		},
		{
			name: "custom dates completes rule when every date has generated reminder",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 6, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:   database.ReminderRuleTypeCustomDates,
					Status: database.ReminderRuleStatusActive,
				},
				CustomDates: []database.CustomReminderDate{
					{RemindDate: base.AddDate(0, 0, 2), GeneratedReminderID: &nextCustomID},
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{next: database.ReminderCompletionNext{CompleteRule: true}},
		},
		{
			name: "invalid interval rule returns error",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 7, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:         database.ReminderRuleTypeInterval,
					IntervalDays: &invalidInterval,
					Status:       database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{expectedErr: expectedInvalidInterval},
		},
		{
			name: "invalid weekdays mask returns error",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 8, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:         database.ReminderRuleTypeWeekdays,
					WeekdaysMask: &invalidMask,
					Status:       database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{expectedErr: expectedInvalidMask},
		},
		{
			name: "unsupported rule returns error",
			input: database.ReminderCompletionInput{
				Reminder: database.ScheduledReminder{ID: 9, RemindAt: base},
				Rule: &database.ReminderRule{
					Type:   "unsupported",
					Status: database.ReminderRuleStatusActive,
				},
			},
			expected: struct {
				next        database.ReminderCompletionNext
				expectedErr string
			}{expectedErr: expectedUnsupportedRule},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			next, err := service.CalculateNextReminder(test.input)

			if test.expected.expectedErr != "" {
				require.EqualError(t, err, test.expected.expectedErr)
				require.Equal(t, test.expected.next, next)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.next, next)
		})
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
