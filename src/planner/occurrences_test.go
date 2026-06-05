package planner_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/planner"
)

func TestCalculateNextOccurrenceTime(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 6, 5, 9, 30, 0, 0, time.Local)
	intervalOne := 1
	intervalTwo := 2
	intervalSeven := 7
	invalidInterval := 0
	workdaysMask := database.WeekdaysMaskWorkdays
	tuesdayThursdayMask := database.WeekdayMaskTuesday | database.WeekdayMaskThursday
	invalidMask := 0
	generatedID := int64(10)

	tests := []struct {
		name            string
		currentRemindAt time.Time
		rule            database.ReminderRule
		customDates     []database.CustomReminderDate
		expected        struct {
			next        planner.NextOccurrence
			expectedErr error
		}
	}{
		{
			name:            "daily interval creates next occurrence from previous remind at plus one day",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeInterval,
				IntervalDays: &intervalOne,
				Status:       database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{next: planner.NextOccurrence{RemindAt: ptrTime(base.AddDate(0, 0, 1))}},
		},
		{
			name:            "two day interval creates next occurrence from previous remind at plus two days",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeInterval,
				IntervalDays: &intervalTwo,
				Status:       database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{next: planner.NextOccurrence{RemindAt: ptrTime(base.AddDate(0, 0, 2))}},
		},
		{
			name:            "weekly interval creates next occurrence from previous remind at plus seven days",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeInterval,
				IntervalDays: &intervalSeven,
				Status:       database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{next: planner.NextOccurrence{RemindAt: ptrTime(base.AddDate(0, 0, 7))}},
		},
		{
			name:            "workdays rule creates monday occurrence after friday",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeWeekdays,
				WeekdaysMask: &workdaysMask,
				Status:       database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{next: planner.NextOccurrence{RemindAt: ptrTime(base.AddDate(0, 0, 3))}},
		},
		{
			name:            "selected weekdays rule creates next occurrence using weekday mask",
			currentRemindAt: time.Date(2026, 6, 2, 9, 30, 0, 0, time.Local),
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeWeekdays,
				WeekdaysMask: &tuesdayThursdayMask,
				Status:       database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{
				next: planner.NextOccurrence{
					RemindAt: ptrTime(time.Date(2026, 6, 4, 9, 30, 0, 0, time.Local)),
				},
			},
		},
		{
			name:            "custom dates creates next occurrence for first date without generated reminder",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:   database.ReminderRuleTypeCustomDates,
				Status: database.ReminderRuleStatusActive,
			},
			customDates: []database.CustomReminderDate{
				{RemindDate: base.AddDate(0, 0, 2), GeneratedReminderID: &generatedID},
				{RemindDate: base.AddDate(0, 0, 4)},
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{next: planner.NextOccurrence{RemindAt: ptrTime(base.AddDate(0, 0, 4))}},
		},
		{
			name:            "custom dates completes rule when every date has generated reminder",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:   database.ReminderRuleTypeCustomDates,
				Status: database.ReminderRuleStatusActive,
			},
			customDates: []database.CustomReminderDate{
				{RemindDate: base.AddDate(0, 0, 2), GeneratedReminderID: &generatedID},
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{next: planner.NextOccurrence{CompleteRule: true}},
		},
		{
			name:            "completed rule does not create next occurrence",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeInterval,
				IntervalDays: &intervalOne,
				Status:       database.ReminderRuleStatusCompleted,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{},
		},
		{
			name:            "invalid interval rule returns error",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeInterval,
				IntervalDays: &invalidInterval,
				Status:       database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{expectedErr: planner.ErrInvalidIntervalRule},
		},
		{
			name:            "invalid weekdays mask returns error",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:         database.ReminderRuleTypeWeekdays,
				WeekdaysMask: &invalidMask,
				Status:       database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{expectedErr: planner.ErrInvalidWeekdaysRule},
		},
		{
			name:            "unsupported rule returns error",
			currentRemindAt: base,
			rule: database.ReminderRule{
				Type:   "unsupported",
				Status: database.ReminderRuleStatusActive,
			},
			expected: struct {
				next        planner.NextOccurrence
				expectedErr error
			}{expectedErr: planner.ErrUnsupportedRuleType},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			next, err := planner.CalculateNextOccurrenceTime(test.currentRemindAt, test.rule, test.customDates)

			if test.expected.expectedErr != nil {
				require.ErrorIs(t, err, test.expected.expectedErr)
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
