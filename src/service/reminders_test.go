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
		before   func(repository *mocks.MockReminderCommandRepository)
		expected struct {
			reminder    database.ReminderView
			expectedErr error
		}
	}{
		{
			name:  "creates reminder with description",
			title: "Pay rent",
			desc:  "Before lunch",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateReminder(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: 1, Title: "Pay rent", Description: "Before lunch"}, nil)
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateReminder(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: 2, Title: "Pay rent", Description: "Before lunch"}, nil)
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateReminder(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: 3, Title: "Pay rent", Description: "Before lunch"}, nil)
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateReminder(gomock.Any(), chatID, "Pay rent", "", remindAt).
					Return(database.ReminderView{ID: 4, Title: "Pay rent"}, nil)
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
			before: func(_ *mocks.MockReminderCommandRepository) {
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateReminder(gomock.Any(), chatID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{}, errCreateReminderRepository)
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
			test.before(repository)

			reminderService := service.NewReminderService(repository)
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
		before       func(repository *mocks.MockReminderCommandRepository)
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateRecurringReminder(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						require.Equal(t, database.WeekdaysMaskWorkdays, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 1, Title: "Work standup"}, nil
					})
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateRecurringReminder(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						require.Equal(t, 2|8, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 2, Title: "Training"}, nil
					})
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateRecurringReminder(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						require.Equal(t, 32|64, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 3, Title: "Weekend plan"}, nil
					})
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
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateRecurringReminder(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request database.RecurringReminderRequest) (database.ReminderView, error) {
						require.Equal(t, 64, *request.Rule.WeekdaysMask)

						return database.ReminderView{ID: 4, Title: "Review"}, nil
					})
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
			before: func(_ *mocks.MockReminderCommandRepository) {
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
			before: func(_ *mocks.MockReminderCommandRepository) {
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
			test.before(repository)

			reminderService := service.NewReminderService(repository)
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
					FindRemindersByChat(gomock.Any(), chatID).
					Return([]database.ReminderView{{ID: 1, Title: "Pay rent"}}, nil)
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
					FindRemindersByChat(gomock.Any(), chatID).
					Return([]database.ReminderView{{ID: 1}, {ID: 2}}, nil)
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
					FindRemindersByChat(gomock.Any(), chatID).
					Return([]database.ReminderView{}, nil)
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
					FindRemindersByChat(gomock.Any(), chatID).
					Return([]database.ReminderView{{ID: 2}, {ID: 1}}, nil)
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
					FindRemindersByChat(gomock.Any(), chatID).
					Return(nil, errFindRemindersRepository)
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
					FindRemindersByChat(gomock.Any(), chatID).
					Return(nil, nil)
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

			reminderService := service.NewReminderService(repository)
			reminders, err := reminderService.Reminders(context.Background(), chatID)

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
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(1)).Return(nil)
			},
		},
		{
			name:       "deletes second reminder",
			reminderID: 2,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(2)).Return(nil)
			},
		},
		{
			name:       "deletes large reminder id",
			reminderID: 999,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(999)).Return(nil)
			},
		},
		{
			name:       "passes chat id to repository",
			reminderID: 3,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(3)).Return(nil)
			},
		},
		{
			name:       "repository cancel error is returned",
			reminderID: 4,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(4)).Return(errCancelReminderRepository)
			},
			expected: struct{ expectedErr error }{
				expectedErr: errExpectedCancelReminder,
			},
		},
		{
			name:       "not found error is returned",
			reminderID: 5,
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().CancelReminder(gomock.Any(), chatID, int64(5)).Return(database.ErrReminderNotFound)
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

			reminderService := service.NewReminderService(repository)
			err := reminderService.DeleteReminder(context.Background(), chatID, test.reminderID)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())

				return
			}

			require.NoError(t, err)
		})
	}
}
