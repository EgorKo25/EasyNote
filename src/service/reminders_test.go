package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	errCreateReminderRepository = errors.New("create reminder repository")
	errFindRemindersRepository  = errors.New("find reminders repository")
	errCancelReminderRepository = errors.New("cancel reminder repository")
	errExpectedTitleEmpty       = errors.New("create reminder: reminder title is empty")
	errExpectedCreateReminder   = errors.New("create reminder: create reminder repository")
	errExpectedFindReminders    = errors.New("find reminders: find reminders repository")
	errExpectedCancelReminder   = errors.New("delete reminder: cancel reminder repository")
	errExpectedReminderNotFound = errors.New("delete reminder: reminder not found")
)

func TestReminderServiceCreatesReminder(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	remindAt := time.Date(2026, 6, 2, 9, 30, 0, 0, time.Local)

	tests := []struct {
		name     string
		title    string
		desc     string
		before   func(planner *mocks.MockReminderCreationPlanner)
		expected struct {
			reminder    database.ReminderView
			expectedErr error
		}
	}{
		{
			name:  "creates reminder with description",
			title: "Pay rent",
			desc:  "Before lunch",
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateSingleOccurrence(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: 1, Title: "Pay rent", Description: "Before lunch"}, nil).
					Times(1)
			},
			expected: struct {
				reminder    database.ReminderView
				expectedErr error
			}{reminder: database.ReminderView{ID: 1, Title: "Pay rent", Description: "Before lunch"}},
		},
		{
			name:  "trims title before create",
			title: "  Pay rent  ",
			desc:  "Before lunch",
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateSingleOccurrence(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: 2, Title: "Pay rent", Description: "Before lunch"}, nil).
					Times(1)
			},
			expected: struct {
				reminder    database.ReminderView
				expectedErr error
			}{reminder: database.ReminderView{ID: 2, Title: "Pay rent", Description: "Before lunch"}},
		},
		{
			name:  "trims description before create",
			title: "Pay rent",
			desc:  "  Before lunch  ",
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateSingleOccurrence(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: 3, Title: "Pay rent", Description: "Before lunch"}, nil).
					Times(1)
			},
			expected: struct {
				reminder    database.ReminderView
				expectedErr error
			}{reminder: database.ReminderView{ID: 3, Title: "Pay rent", Description: "Before lunch"}},
		},
		{
			name:  "creates reminder without description",
			title: "Pay rent",
			desc:  "",
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateSingleOccurrence(gomock.Any(), chatID, "Pay rent", "", remindAt).
					Return(database.ReminderView{ID: 4, Title: "Pay rent"}, nil).
					Times(1)
			},
			expected: struct {
				reminder    database.ReminderView
				expectedErr error
			}{reminder: database.ReminderView{ID: 4, Title: "Pay rent"}},
		},
		{
			name:  "empty title returns validation error",
			title: " ",
			desc:  "Before lunch",
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder    database.ReminderView
				expectedErr error
			}{expectedErr: errExpectedTitleEmpty},
		},
		{
			name:  "repository create error is returned",
			title: "Pay rent",
			desc:  "Before lunch",
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateSingleOccurrence(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{}, errCreateReminderRepository).
					Times(1)
			},
			expected: struct {
				reminder    database.ReminderView
				expectedErr error
			}{expectedErr: errExpectedCreateReminder},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			planner := mocks.NewMockReminderCreationPlanner(ctrl)
			test.before(planner)

			reminderService := service.NewReminderService(repository, planner)
			reminder, err := reminderService.CreateReminder(
				context.Background(),
				chatID,
				test.title,
				test.desc,
				remindAt,
			)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())
				require.Equal(t, test.expected.reminder, reminder)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.reminder, reminder)
		})
	}
}

func TestReminderServiceCreatesWeekdaysReminder(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	remindAt := time.Date(2026, 6, 2, 9, 30, 0, 0, time.Local)

	tests := []struct {
		name         string
		title        string
		desc         string
		weekdaysMask int
		before       func(planner *mocks.MockReminderCreationPlanner)
		expected     struct {
			reminder        database.ReminderView
			expectedErrText string
		}
	}{
		{
			name:         "creates workdays reminder with default workdays mask",
			title:        "Work standup",
			desc:         "Daily sync",
			weekdaysMask: database.WeekdaysMaskWorkdays,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, database.WeekdaysMaskWorkdays, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 1, Title: "Work standup"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 1, Title: "Work standup"}},
		},
		{
			name:         "creates tuesday and thursday reminder with custom mask",
			title:        "Training",
			desc:         "Gym",
			weekdaysMask: 2 | 8,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, 2|8, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 2, Title: "Training"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 2, Title: "Training"}},
		},
		{
			name:         "creates weekend reminder with saturday and sunday mask",
			title:        "Weekend plan",
			desc:         "",
			weekdaysMask: 32 | 64,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, 32|64, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 3, Title: "Weekend plan"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 3, Title: "Weekend plan"}},
		},
		{
			name:         "creates sunday only reminder with single day mask",
			title:        "Review",
			desc:         "Weekly",
			weekdaysMask: 64,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, 64, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 4, Title: "Review"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 4, Title: "Review"}},
		},
		{
			name:         "empty title returns validation error",
			title:        " ",
			desc:         "Weekly",
			weekdaysMask: 64,
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "create weekdays reminder: reminder title is empty"},
		},
		{
			name:         "empty weekdays mask returns validation error",
			title:        "Review",
			desc:         "Weekly",
			weekdaysMask: 0,
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "create weekdays reminder: weekdays mask is empty"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			planner := mocks.NewMockReminderCreationPlanner(ctrl)
			test.before(planner)

			reminderService := service.NewReminderService(repository, planner)
			reminder, err := reminderService.CreateWeekdaysReminder(
				context.Background(),
				chatID,
				test.title,
				test.desc,
				remindAt,
				test.weekdaysMask,
			)

			if test.expected.expectedErrText != "" {
				require.EqualError(t, err, test.expected.expectedErrText)
				require.Equal(t, test.expected.reminder, reminder)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.reminder, reminder)
		})
	}
}

func TestReminderServiceCreatesIntervalReminder(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	remindAt := time.Date(2026, 6, 2, 9, 30, 0, 0, time.Local)

	tests := []struct {
		name         string
		title        string
		desc         string
		intervalDays int
		before       func(planner *mocks.MockReminderCreationPlanner)
		expected     struct {
			reminder        database.ReminderView
			expectedErrText string
		}
	}{
		{
			name:         "creates daily interval reminder",
			title:        "Medicine",
			desc:         "Morning",
			intervalDays: 1,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, database.ReminderRuleTypeInterval, request.Rule.Type)
						require.Equal(t, 1, *request.Rule.IntervalDays)

						return database.ReminderView{ID: 10, Title: "Medicine"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 10, Title: "Medicine"}},
		},
		{
			name:         "creates two day interval reminder",
			title:        "Water plants",
			desc:         "",
			intervalDays: 2,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, 2, *request.Rule.IntervalDays)

						return database.ReminderView{ID: 11, Title: "Water plants"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 11, Title: "Water plants"}},
		},
		{
			name:         "creates weekly interval reminder",
			title:        "Report",
			desc:         "Team",
			intervalDays: 7,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, 7, *request.Rule.IntervalDays)
						require.Equal(t, "Team", request.Description)

						return database.ReminderView{ID: 12, Title: "Report"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 12, Title: "Report"}},
		},
		{
			name:         "trims interval reminder title and description",
			title:        "  Pay rent  ",
			desc:         "  Card  ",
			intervalDays: 1,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, "Pay rent", request.Title)
						require.Equal(t, "Card", request.Description)

						return database.ReminderView{ID: 13, Title: "Pay rent", Description: "Card"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 13, Title: "Pay rent", Description: "Card"}},
		},
		{
			name:         "empty interval title returns validation error",
			title:        " ",
			desc:         "Morning",
			intervalDays: 1,
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "create recurring reminder: reminder title is empty"},
		},
		{
			name:         "planner storage interval error is returned",
			title:        "Medicine",
			desc:         "Morning",
			intervalDays: 1,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					Return(database.ReminderView{}, errCreateReminderRepository).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "create recurring reminder: create reminder repository"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			planner := mocks.NewMockReminderCreationPlanner(ctrl)
			test.before(planner)

			reminderService := service.NewReminderService(repository, planner)
			reminder, err := reminderService.CreateIntervalReminder(
				context.Background(),
				chatID,
				test.title,
				test.desc,
				remindAt,
				test.intervalDays,
			)

			if test.expected.expectedErrText != "" {
				require.EqualError(t, err, test.expected.expectedErrText)
				require.Equal(t, test.expected.reminder, reminder)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.reminder, reminder)
		})
	}
}

func TestReminderServiceCreatesCustomDatesReminder(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	location := time.Local
	firstDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	secondDate := time.Date(2026, 6, 8, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name          string
		title         string
		desc          string
		selectedDates []time.Time
		hour          int
		minute        int
		before        func(planner *mocks.MockReminderCreationPlanner)
		expected      struct {
			reminder        database.ReminderView
			expectedErrText string
		}
	}{
		{
			name:          "creates custom dates reminder using earliest selected date",
			title:         "Trip",
			desc:          "Pack",
			selectedDates: []time.Time{firstDate, secondDate},
			hour:          8,
			minute:        15,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, database.ReminderRuleTypeCustomDates, request.Rule.Type)
						require.Equal(t, time.Date(2026, 6, 8, 8, 15, 0, 0, location), request.FirstRemindAt)
						require.Equal(t, []time.Time{firstDate, secondDate}, request.CustomDates)

						return database.ReminderView{ID: 20, Title: "Trip"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 20, Title: "Trip"}},
		},
		{
			name:          "creates custom dates reminder with one selected date",
			title:         "Doctor",
			desc:          "",
			selectedDates: []time.Time{firstDate},
			hour:          12,
			minute:        0,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, time.Date(2026, 6, 10, 12, 0, 0, 0, location), request.FirstRemindAt)

						return database.ReminderView{ID: 21, Title: "Doctor"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 21, Title: "Doctor"}},
		},
		{
			name:          "creates custom dates reminder and trims text",
			title:         "  Tax  ",
			desc:          "  Forms  ",
			selectedDates: []time.Time{firstDate},
			hour:          18,
			minute:        45,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, "Tax", request.Title)
						require.Equal(t, "Forms", request.Description)

						return database.ReminderView{ID: 22, Title: "Tax", Description: "Forms"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 22, Title: "Tax", Description: "Forms"}},
		},
		{
			name:          "creates custom dates reminder preserving selected date order",
			title:         "Tickets",
			desc:          "Buy",
			selectedDates: []time.Time{secondDate, firstDate},
			hour:          9,
			minute:        5,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						t.Helper()
						require.Equal(t, []time.Time{secondDate, firstDate}, request.CustomDates)

						return database.ReminderView{ID: 23, Title: "Tickets"}, nil
					}).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 23, Title: "Tickets"}},
		},
		{
			name:          "empty custom dates returns validation error",
			title:         "Trip",
			desc:          "Pack",
			selectedDates: []time.Time{},
			hour:          8,
			minute:        15,
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "create custom dates reminder: custom reminder dates are empty"},
		},
		{
			name:          "planner storage custom dates error is returned",
			title:         "Trip",
			desc:          "Pack",
			selectedDates: []time.Time{firstDate},
			hour:          8,
			minute:        15,
			before: func(planner *mocks.MockReminderCreationPlanner) {
				planner.EXPECT().
					CreateInitialOccurrenceForRule(gomock.Any(), gomock.Any()).
					Return(database.ReminderView{}, errCreateReminderRepository).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "create recurring reminder: create reminder repository"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			planner := mocks.NewMockReminderCreationPlanner(ctrl)
			test.before(planner)

			reminderService := service.NewReminderService(repository, planner)
			reminder, err := reminderService.CreateCustomDatesReminder(
				context.Background(),
				chatID,
				test.title,
				test.desc,
				test.selectedDates,
				test.hour,
				test.minute,
				location,
			)

			if test.expected.expectedErrText != "" {
				require.EqualError(t, err, test.expected.expectedErrText)
				require.Equal(t, test.expected.reminder, reminder)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.reminder, reminder)
		})
	}
}

func TestReminderServiceLoadsReminders(t *testing.T) {
	t.Parallel()

	chatID := int64(100)

	tests := []struct {
		name     string
		before   func(repository *mocks.MockReminderCommandRepository)
		expected struct {
			reminders   []database.ReminderView
			expectedErr error
		}
	}{
		{
			name: "loads one reminder",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, 2, 4).
					Return([]database.ReminderView{{ID: 1, Title: "Pay rent"}}, nil).
					Times(1)
			},
			expected: struct {
				reminders   []database.ReminderView
				expectedErr error
			}{reminders: []database.ReminderView{{ID: 1, Title: "Pay rent"}}},
		},
		{
			name: "loads several reminders",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, 2, 4).
					Return([]database.ReminderView{{ID: 1}, {ID: 2}}, nil).
					Times(1)
			},
			expected: struct {
				reminders   []database.ReminderView
				expectedErr error
			}{reminders: []database.ReminderView{{ID: 1}, {ID: 2}}},
		},
		{
			name: "loads empty reminder list",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, 2, 4).
					Return([]database.ReminderView{}, nil).
					Times(1)
			},
			expected: struct {
				reminders   []database.ReminderView
				expectedErr error
			}{reminders: []database.ReminderView{}},
		},
		{
			name: "keeps repository reminder order",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, 2, 4).
					Return([]database.ReminderView{{ID: 2}, {ID: 1}}, nil).
					Times(1)
			},
			expected: struct {
				reminders   []database.ReminderView
				expectedErr error
			}{reminders: []database.ReminderView{{ID: 2}, {ID: 1}}},
		},
		{
			name: "repository find error is returned",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, 2, 4).
					Return(nil, errFindRemindersRepository).
					Times(1)
			},
			expected: struct {
				reminders   []database.ReminderView
				expectedErr error
			}{expectedErr: errExpectedFindReminders},
		},
		{
			name: "nil reminder list is returned explicitly",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, 2, 4).
					Return(nil, nil).
					Times(1)
			},
			expected: struct {
				reminders   []database.ReminderView
				expectedErr error
			}{reminders: nil},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			test.before(repository)

			reminderService := service.NewReminderService(repository, mocks.NewMockReminderCreationPlanner(ctrl))
			reminders, err := reminderService.Reminders(context.Background(), chatID, 2, 4)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())
				require.Equal(t, test.expected.reminders, reminders)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.reminders, reminders)
		})
	}
}

func TestReminderServiceDeletesReminder(t *testing.T) {
	t.Parallel()

	chatID := int64(100)

	tests := []struct {
		name       string
		reminderID int64
		before     func(repository *mocks.MockReminderCommandRepository)
		expected   struct {
			expectedErr error
		}
	}{
		{
			name:       "deletes first reminder",
			reminderID: 1,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(1)).Return(nil).
					Times(1)
			},
		},
		{
			name:       "deletes second reminder",
			reminderID: 2,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(2)).Return(nil).
					Times(1)
			},
		},
		{
			name:       "deletes large reminder id",
			reminderID: 999,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(999)).Return(nil).
					Times(1)
			},
		},
		{
			name:       "passes chat id to repository",
			reminderID: 3,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(3)).Return(nil).
					Times(1)
			},
		},
		{
			name:       "repository cancel error is returned",
			reminderID: 4,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(4)).Return(errCancelReminderRepository).
					Times(1)
			},
			expected: struct{ expectedErr error }{
				expectedErr: errExpectedCancelReminder,
			},
		},
		{
			name:       "not found error is returned",
			reminderID: 5,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(5)).Return(database.ErrReminderNotFound).
					Times(1)
			},
			expected: struct{ expectedErr error }{
				expectedErr: errExpectedReminderNotFound,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			test.before(repository)

			reminderService := service.NewReminderService(repository, mocks.NewMockReminderCreationPlanner(ctrl))
			err := reminderService.DeleteReminder(context.Background(), chatID, test.reminderID)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestReminderServiceLoadsReminderHistory(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	limit := 2
	offset := 4

	tests := []struct {
		name     string
		before   func(repository *mocks.MockReminderCommandRepository)
		expected struct {
			reminders       []database.ReminderView
			expectedErrText string
		}
	}{
		{
			name: "loads completed reminder history page",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, limit, offset).
					Return([]database.ReminderView{{ID: 1, Status: database.ReminderStatusCompleted}}, nil).
					Times(1)
			},
			expected: struct {
				reminders       []database.ReminderView
				expectedErrText string
			}{reminders: []database.ReminderView{{ID: 1, Status: database.ReminderStatusCompleted}}},
		},
		{
			name: "loads canceled reminder history page",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, limit, offset).
					Return([]database.ReminderView{{ID: 2, Status: database.ReminderStatusCanceled}}, nil).
					Times(1)
			},
			expected: struct {
				reminders       []database.ReminderView
				expectedErrText string
			}{reminders: []database.ReminderView{{ID: 2, Status: database.ReminderStatusCanceled}}},
		},
		{
			name: "loads empty reminder history page",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, limit, offset).
					Return([]database.ReminderView{}, nil).
					Times(1)
			},
			expected: struct {
				reminders       []database.ReminderView
				expectedErrText string
			}{reminders: []database.ReminderView{}},
		},
		{
			name: "keeps reminder history repository order",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, limit, offset).
					Return([]database.ReminderView{{ID: 4}, {ID: 3}}, nil).
					Times(1)
			},
			expected: struct {
				reminders       []database.ReminderView
				expectedErrText string
			}{reminders: []database.ReminderView{{ID: 4}, {ID: 3}}},
		},
		{
			name: "repository history error is returned",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, limit, offset).
					Return(nil, errFindRemindersRepository).
					Times(1)
			},
			expected: struct {
				reminders       []database.ReminderView
				expectedErrText string
			}{expectedErrText: "find reminder history: find reminders repository"},
		},
		{
			name: "nil reminder history list is returned explicitly",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, limit, offset).
					Return(nil, nil).
					Times(1)
			},
			expected: struct {
				reminders       []database.ReminderView
				expectedErrText string
			}{reminders: nil},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			test.before(repository)

			reminderService := service.NewReminderService(repository, mocks.NewMockReminderCreationPlanner(ctrl))
			reminders, err := reminderService.ReminderHistory(context.Background(), chatID, limit, offset)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected reminder history error")
				require.Equal(t, test.expected.reminders, reminders)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.reminders, reminders)
		})
	}
}

func TestReminderServiceUpdatesUserSettings(t *testing.T) {
	t.Parallel()

	chatID := int64(100)

	tests := []struct {
		name     string
		action   func(reminderService *service.ReminderService) error
		before   func(repository *mocks.MockReminderCommandRepository)
		expected struct {
			expectedErrText string
		}
	}{
		{
			name: "sets valid moscow timezone",
			action: func(reminderService *service.ReminderService) error {
				return reminderService.SetUserTimezone(context.Background(), chatID, "Europe/Moscow")
			},
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().UpdateUserTimezone(gomock.Any(), chatID, "Europe/Moscow").Return(nil).Times(1)
			},
		},
		{
			name: "sets valid utc timezone with surrounding spaces",
			action: func(reminderService *service.ReminderService) error {
				return reminderService.SetUserTimezone(context.Background(), chatID, " UTC ")
			},
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().UpdateUserTimezone(gomock.Any(), chatID, "UTC").Return(nil).Times(1)
			},
		},
		{
			name: "sets enabled intrusive settings at lower boundary",
			action: func(reminderService *service.ReminderService) error {
				return reminderService.SetIntrusiveSettings(context.Background(), chatID, true, time.Minute)
			},
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateIntrusiveSettings(gomock.Any(), chatID, true, time.Minute).
					Return(nil).
					Times(1)
			},
		},
		{
			name: "sets disabled intrusive settings at upper boundary",
			action: func(reminderService *service.ReminderService) error {
				return reminderService.SetIntrusiveSettings(context.Background(), chatID, false, 2*time.Hour)
			},
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateIntrusiveSettings(gomock.Any(), chatID, false, 2*time.Hour).
					Return(nil).
					Times(1)
			},
		},
		{
			name: "rejects blank timezone",
			action: func(reminderService *service.ReminderService) error {
				return reminderService.SetUserTimezone(context.Background(), chatID, " ")
			},
			before: func(_ *mocks.MockReminderCommandRepository) {
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: "unknown timezone: "},
		},
		{
			name: "rejects intrusive interval below lower boundary",
			action: func(reminderService *service.ReminderService) error {
				return reminderService.SetIntrusiveSettings(context.Background(), chatID, true, time.Second)
			},
			before: func(_ *mocks.MockReminderCommandRepository) {
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: "set intrusive settings: intrusive reminder interval is out of range"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			test.before(repository)

			reminderService := service.NewReminderService(repository, mocks.NewMockReminderCreationPlanner(ctrl))
			err := test.action(reminderService)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected settings update error")

				return
			}

			require.NoError(t, err)
		})
	}
}
