package planner_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/planner"
	plannerMocks "github.com/EgorKo25/EasyNote/src/planner/mocks"
)

var errPlannerStorage = errors.New("planner storage")

func TestPlannerCompletesOccurrenceLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := time.Date(2026, 6, 5, 9, 30, 0, 0, time.Local)
	sentAt := base.Add(time.Minute)
	ruleID := int64(20)
	intervalOne := 1
	workdaysMask := database.WeekdaysMaskWorkdays
	generatedID := int64(99)

	tests := []struct {
		name     string
		action   func(ctx context.Context, schedule *planner.Planner) error
		before   func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage)
		expected struct {
			expectedErr error
		}
	}{
		{
			name: "sent interval occurrence creates next occurrence from old remind at",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 1, database.ReminderStatusSent, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				occurrence := database.Reminder{
					ID: 1, NoteID: 10, RuleID: &ruleID, RemindAt: base, Status: database.ReminderStatusNew,
				}
				rule := database.ReminderRule{
					ID: ruleID, Type: database.ReminderRuleTypeInterval, IntervalDays: &intervalOne,
					Status: database.ReminderRuleStatusActive,
				}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(1)).
						Return(occurrence, nil).
						Times(1),
					transaction.EXPECT().
						MarkOccurrence(gomock.Any(), int64(1), database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
					transaction.EXPECT().
						LoadRuleForUpdate(gomock.Any(), ruleID).
						Return(rule, nil).
						Times(1),
					transaction.EXPECT().
						CreateNextOccurrence(gomock.Any(), occurrence.NoteID, ruleID, base.AddDate(0, 0, 1)).
						Return(database.Reminder{ID: 100}, nil).
						Times(1),
				)
			},
		},
		{
			name: "failed weekdays occurrence creates monday after friday",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 2, database.ReminderStatusFailed, nil)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				occurrence := database.Reminder{
					ID: 2, NoteID: 10, RuleID: &ruleID, RemindAt: base, Status: database.ReminderStatusNew,
				}
				rule := database.ReminderRule{
					ID: ruleID, Type: database.ReminderRuleTypeWeekdays, WeekdaysMask: &workdaysMask,
					Status: database.ReminderRuleStatusActive,
				}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(2)).
						Return(occurrence, nil).
						Times(1),
					transaction.EXPECT().
						MarkOccurrence(gomock.Any(), int64(2), database.ReminderStatusFailed, nil).
						Return(nil).
						Times(1),
					transaction.EXPECT().
						LoadRuleForUpdate(gomock.Any(), ruleID).
						Return(rule, nil).
						Times(1),
					transaction.EXPECT().
						CreateNextOccurrence(gomock.Any(), occurrence.NoteID, ruleID, base.AddDate(0, 0, 3)).
						Return(database.Reminder{ID: 101}, nil).
						Times(1),
				)
			},
		},
		{
			name: "custom dates occurrence creates next selected date and marks it generated",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 3, database.ReminderStatusSent, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				nextRemindAt := base.AddDate(0, 0, 4)
				occurrence := database.Reminder{
					ID: 3, NoteID: 10, RuleID: &ruleID, RemindAt: base, Status: database.ReminderStatusNew,
				}
				rule := database.ReminderRule{
					ID: ruleID, Type: database.ReminderRuleTypeCustomDates, Status: database.ReminderRuleStatusActive,
				}
				customDates := []database.CustomReminderDate{
					{RuleID: ruleID, RemindDate: base, GeneratedReminderID: &generatedID},
					{RuleID: ruleID, RemindDate: nextRemindAt},
				}
				created := database.Reminder{ID: 30, NoteID: occurrence.NoteID, RuleID: &ruleID, RemindAt: nextRemindAt}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(3)).
						Return(occurrence, nil).
						Times(1),
					transaction.EXPECT().
						MarkOccurrence(gomock.Any(), int64(3), database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
					transaction.EXPECT().
						LoadRuleForUpdate(gomock.Any(), ruleID).
						Return(rule, nil).
						Times(1),
					transaction.EXPECT().
						LoadCustomDatesForUpdate(gomock.Any(), ruleID).
						Return(customDates, nil).
						Times(1),
					transaction.EXPECT().
						CreateNextOccurrence(gomock.Any(), occurrence.NoteID, ruleID, nextRemindAt).
						Return(created, nil).
						Times(1),
					transaction.EXPECT().
						MarkCustomDateGenerated(gomock.Any(), ruleID, nextRemindAt, created.ID).
						Return(nil).
						Times(1),
				)
			},
		},
		{
			name: "one time occurrence completes without creating next occurrence",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 4, database.ReminderStatusSent, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				occurrence := database.Reminder{
					ID: 4, NoteID: 10, RemindAt: base, Status: database.ReminderStatusNew,
				}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(4)).
						Return(occurrence, nil).
						Times(1),
					transaction.EXPECT().
						MarkOccurrence(gomock.Any(), int64(4), database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
				)
			},
		},
		{
			name: "active holiday occurrence is moved to next year",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteHoliday(ctx, 10, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				holiday := database.Holiday{
					ID: 10, TelegramChatID: 100, HolidayName: "Birthday", RecipientName: "Oleg",
					RemindAt: base, Status: database.HolidayStatusActive,
				}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadHolidayForUpdate(gomock.Any(), int64(10)).
						Return(holiday, nil).
						Times(1),
					transaction.EXPECT().
						MarkHolidayOccurrence(gomock.Any(), holiday, gomock.Any()).
						Return(nil).
						Times(1),
				)
			},
		},
		{
			name: "already completed occurrence is idempotent and creates nothing",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 5, database.ReminderStatusSent, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				occurrence := database.Reminder{
					ID: 5, NoteID: 10, RuleID: &ruleID, RemindAt: base, Status: database.ReminderStatusSent,
				}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(5)).
						Return(occurrence, nil).
						Times(1),
				)
			},
		},
		{
			name: "custom dates occurrence completes rule after last selected date",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 6, database.ReminderStatusSent, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				occurrence := database.Reminder{
					ID: 6, NoteID: 10, RuleID: &ruleID, RemindAt: base, Status: database.ReminderStatusNew,
				}
				rule := database.ReminderRule{
					ID: ruleID, Type: database.ReminderRuleTypeCustomDates, Status: database.ReminderRuleStatusActive,
				}
				customDates := []database.CustomReminderDate{
					{RuleID: ruleID, RemindDate: base, GeneratedReminderID: &generatedID},
				}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(6)).
						Return(occurrence, nil).
						Times(1),
					transaction.EXPECT().
						MarkOccurrence(gomock.Any(), int64(6), database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
					transaction.EXPECT().
						LoadRuleForUpdate(gomock.Any(), ruleID).
						Return(rule, nil).
						Times(1),
					transaction.EXPECT().
						LoadCustomDatesForUpdate(gomock.Any(), ruleID).
						Return(customDates, nil).
						Times(1),
					transaction.EXPECT().
						UpdateRuleStatus(gomock.Any(), ruleID, database.ReminderRuleStatusCompleted).
						Return(nil).
						Times(1),
				)
			},
		},
		{
			name: "storage load error is returned",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 7, database.ReminderStatusSent, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(7)).
						Return(database.Reminder{}, errPlannerStorage).
						Times(1),
				)
			},
			expected: struct {
				expectedErr error
			}{expectedErr: errPlannerStorage},
		},
		{
			name: "invalid active rule error is returned after occurrence is marked sent",
			action: func(ctx context.Context, schedule *planner.Planner) error {
				return schedule.CompleteOccurrence(ctx, 8, database.ReminderStatusSent, &sentAt)
			},
			before: func(storage *plannerMocks.MockStorage, transaction *plannerMocks.MockTransactionStorage) {
				invalidInterval := 0
				occurrence := database.Reminder{
					ID: 8, NoteID: 10, RuleID: &ruleID, RemindAt: base, Status: database.ReminderStatusNew,
				}
				rule := database.ReminderRule{
					ID: ruleID, Type: database.ReminderRuleTypeInterval, IntervalDays: &invalidInterval,
					Status: database.ReminderRuleStatusActive,
				}

				gomock.InOrder(
					storage.EXPECT().
						WithinTransaction(gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, action func(context.Context, planner.TransactionStorage) error) error {
							return action(ctx, transaction)
						}).
						Times(1),
					transaction.EXPECT().
						LoadOccurrenceForUpdate(gomock.Any(), int64(8)).
						Return(occurrence, nil).
						Times(1),
					transaction.EXPECT().
						MarkOccurrence(gomock.Any(), int64(8), database.ReminderStatusSent, gomock.Any()).
						Return(nil).
						Times(1),
					transaction.EXPECT().
						LoadRuleForUpdate(gomock.Any(), ruleID).
						Return(rule, nil).
						Times(1),
				)
			},
			expected: struct {
				expectedErr error
			}{expectedErr: planner.ErrInvalidIntervalRule},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			storage := plannerMocks.NewMockStorage(ctrl)
			transaction := plannerMocks.NewMockTransactionStorage(ctrl)
			test.before(storage, transaction)
			schedule := planner.New(storage)

			err := test.action(ctx, schedule)

			if test.expected.expectedErr != nil {
				require.ErrorIs(t, err, test.expected.expectedErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
