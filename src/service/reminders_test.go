//nolint:goconst,wrapcheck,err113 // Tests intentionally keep repeated fixture values and direct action errors.
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/domain"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	errCreateReminderRepository = errors.New("create reminder repository")
	errCancelReminderRepository = errors.New("cancel reminder repository")
	errHolidayRepository        = errors.New("holiday repository")
	errShoppingRepository       = errors.New("shopping repository")
	errUpdateReminderRepository = errors.New("update reminder repository")
	errExpectedTitleEmpty       = errors.New("create reminder: reminder title is empty")
	errExpectedCreateReminder   = errors.New("create reminder: create reminder repository")
	errExpectedCancelReminder   = errors.New("delete reminder: cancel reminder repository")
	errExpectedReminderNotFound = errors.New("delete reminder: reminder not found")
)

func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

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
			reminder    domain.ReminderView
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
				reminder    domain.ReminderView
				expectedErr error
			}{reminder: domain.ReminderView{ID: 1, Title: "Pay rent", Description: "Before lunch"}},
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
				reminder    domain.ReminderView
				expectedErr error
			}{reminder: domain.ReminderView{ID: 2, Title: "Pay rent", Description: "Before lunch"}},
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
				reminder    domain.ReminderView
				expectedErr error
			}{reminder: domain.ReminderView{ID: 3, Title: "Pay rent", Description: "Before lunch"}},
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
				reminder    domain.ReminderView
				expectedErr error
			}{reminder: domain.ReminderView{ID: 4, Title: "Pay rent"}},
		},
		{
			name:  "empty title returns validation error",
			title: " ",
			desc:  "Before lunch",
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder    domain.ReminderView
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
				reminder    domain.ReminderView
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

func TestReminderServiceUpdatesReminder(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	reminderID := int64(55)
	remindAt := time.Date(2026, 6, 2, 9, 30, 0, 0, time.Local)

	tests := []struct {
		name     string
		title    string
		desc     string
		before   func(repository *mocks.MockReminderCommandRepository)
		expected struct {
			reminder        domain.ReminderView
			expectedErrText string
		}
	}{
		{
			name:  "updates reminder with description",
			title: "Pay rent",
			desc:  "Before lunch",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateReminder(gomock.Any(), chatID, reminderID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: reminderID, Title: "Pay rent", Description: "Before lunch"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: reminderID, Title: "Pay rent", Description: "Before lunch"}},
		},
		{
			name:  "trims title before update",
			title: "  Pay rent  ",
			desc:  "Before lunch",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateReminder(gomock.Any(), chatID, reminderID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: reminderID, Title: "Pay rent", Description: "Before lunch"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: reminderID, Title: "Pay rent", Description: "Before lunch"}},
		},
		{
			name:  "trims description before update",
			title: "Pay rent",
			desc:  "  Before lunch  ",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateReminder(gomock.Any(), chatID, reminderID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{ID: reminderID, Title: "Pay rent", Description: "Before lunch"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: reminderID, Title: "Pay rent", Description: "Before lunch"}},
		},
		{
			name:  "updates reminder without description",
			title: "Pay rent",
			desc:  "",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateReminder(gomock.Any(), chatID, reminderID, "Pay rent", "", remindAt).
					Return(database.ReminderView{ID: reminderID, Title: "Pay rent"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: reminderID, Title: "Pay rent"}},
		},
		{
			name:  "empty title returns validation error",
			title: " ",
			desc:  "Before lunch",
			before: func(_ *mocks.MockReminderCommandRepository) {
			},
			expected: struct {
				reminder        domain.ReminderView
				expectedErrText string
			}{expectedErrText: "update reminder: reminder title is empty"},
		},
		{
			name:  "repository update error is returned",
			title: "Pay rent",
			desc:  "Before lunch",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateReminder(gomock.Any(), chatID, reminderID, "Pay rent", "Before lunch", remindAt).
					Return(database.ReminderView{}, errUpdateReminderRepository).
					Times(1)
			},
			expected: struct {
				reminder        domain.ReminderView
				expectedErrText string
			}{expectedErrText: "update reminder: update reminder repository"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			test.before(repository)

			reminderService := service.NewReminderService(repository, mocks.NewMockReminderCreationPlanner(ctrl))
			reminder, err := reminderService.UpdateReminder(
				context.Background(),
				chatID,
				reminderID,
				test.title,
				test.desc,
				remindAt,
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

func TestReminderServiceManagesHolidays(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	holidayID := int64(7)
	holidayDate := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	remindAt := time.Date(2026, 6, 2, 10, 15, 0, 0, time.UTC)

	tests := []struct {
		name     string
		before   func(repository *mocks.MockReminderCommandRepository)
		action   func(reminderService *service.ReminderService) (string, string)
		expected struct {
			result          string
			expectedErrText string
		}
	}{
		{
			name: "create holiday trims name fields and returns created holiday",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateHoliday(gomock.Any(), database.HolidayRequest{
						TelegramChatID: chatID,
						HolidayName:    "Birthday",
						RecipientName:  "Oleg",
						HolidayDate:    holidayDate,
						RemindAt:       remindAt,
					}).
					Return(database.HolidayView{ID: holidayID, HolidayName: "Birthday"}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				holiday, err := reminderService.CreateHoliday(context.Background(), domain.HolidayRequest{
					TelegramChatID: chatID,
					HolidayName:    " Birthday ",
					RecipientName:  " Oleg ",
					HolidayDate:    holidayDate,
					RemindAt:       remindAt,
				})

				return holiday.HolidayName, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "Birthday"},
		},
		{
			name: "update holiday trims name fields and returns updated holiday",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateHoliday(gomock.Any(), database.HolidayUpdateRequest{
						ID: holidayID,
						HolidayRequest: database.HolidayRequest{
							TelegramChatID: chatID,
							HolidayName:    "Anniversary",
							RecipientName:  "Anna",
							HolidayDate:    holidayDate,
							RemindAt:       remindAt,
						},
					}).
					Return(database.HolidayView{ID: holidayID, HolidayName: "Anniversary"}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				holiday, err := reminderService.UpdateHoliday(context.Background(), domain.HolidayUpdateRequest{
					ID: holidayID,
					HolidayRequest: domain.HolidayRequest{
						TelegramChatID: chatID,
						HolidayName:    " Anniversary ",
						RecipientName:  " Anna ",
						HolidayDate:    holidayDate,
						RemindAt:       remindAt,
					},
				})

				return holiday.HolidayName, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "Anniversary"},
		},
		{
			name: "find holidays returns repository page",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindHolidaysByChat(gomock.Any(), chatID, 2, 4).
					Return([]database.HolidayView{{ID: 1}, {ID: 2}}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				holidays, err := reminderService.FindHolidaysByChat(context.Background(), chatID, 2, 4)
				if len(holidays) == 2 {
					return "two holidays", errorText(err)
				}

				return "unexpected holidays", errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "two holidays"},
		},
		{
			name: "holiday by id returns repository holiday",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					HolidayByID(gomock.Any(), chatID, holidayID).
					Return(database.HolidayView{ID: holidayID, RecipientName: "Oleg"}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				holiday, err := reminderService.HolidayByID(context.Background(), chatID, holidayID)

				return holiday.RecipientName, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "Oleg"},
		},
		{
			name: "delete holiday returns nil after repository delete",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					DeleteHoliday(gomock.Any(), chatID, holidayID).
					Return(nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				err := reminderService.DeleteHoliday(context.Background(), chatID, holidayID)

				return "deleted", errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "deleted"},
		},
		{
			name: "create holiday repository error is wrapped",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateHoliday(gomock.Any(), gomock.Any()).
					Return(database.HolidayView{}, errHolidayRepository).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				holiday, err := reminderService.CreateHoliday(context.Background(), domain.HolidayRequest{
					TelegramChatID: chatID,
					HolidayName:    "Birthday",
					RecipientName:  "Oleg",
					HolidayDate:    holidayDate,
					RemindAt:       remindAt,
				})

				return holiday.HolidayName, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "create holiday: holiday repository"},
		},
		{
			name: "find holidays repository error is wrapped",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindHolidaysByChat(gomock.Any(), chatID, 2, 4).
					Return(nil, errHolidayRepository).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				holidays, err := reminderService.FindHolidaysByChat(context.Background(), chatID, 2, 4)
				if len(holidays) == 0 {
					return "", errorText(err)
				}

				return "unexpected holidays", errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "find holidays by chat: holiday repository"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			planner := mocks.NewMockReminderCreationPlanner(ctrl)

			test.before(repository)

			reminderService := service.NewReminderService(repository, planner)
			result, errText := test.action(reminderService)

			if test.expected.expectedErrText != "" {
				require.Equal(t, test.expected.expectedErrText, errText)
				require.Equal(t, test.expected.result, result)

				return
			}

			require.Empty(t, errText)
			require.Equal(t, test.expected.result, result)
		})
	}
}

func TestReminderServiceManagesShoppingLists(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	listID := int64(9)
	itemID := int64(10)
	items := []database.ShoppingListItem{{Text: "Milk", Position: 1}, {Text: "Bread", Position: 2}}
	unexpectedList := "unexpected list"

	tests := []struct {
		name     string
		before   func(repository *mocks.MockReminderCommandRepository)
		action   func(reminderService *service.ReminderService) (string, string)
		expected struct {
			result          string
			expectedErrText string
		}
	}{
		{
			name: "create shopping list returns repository list",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateShoppingList(gomock.Any(), chatID, items).
					Return(database.ShoppingListView{ID: listID, Items: items}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				list, err := reminderService.CreateShoppingList(context.Background(), chatID, []domain.ShoppingListItem{
					{Text: "Milk", Position: 1},
					{Text: "Bread", Position: 2},
				})
				if len(list.Items) == 2 {
					return "two items", errorText(err)
				}

				return "unexpected items", errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "two items"},
		},
		{
			name: "find shopping lists returns repository page",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindShoppingListsByChat(gomock.Any(), chatID, 2, 4).
					Return([]database.ShoppingListView{{ID: 1}, {ID: 2}}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				lists, err := reminderService.FindShoppingListsByChat(context.Background(), chatID, 2, 4)
				if len(lists) == 2 {
					return "two lists", errorText(err)
				}

				return "unexpected lists", errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "two lists"},
		},
		{
			name: "shopping list by id returns repository list",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					ShoppingListByID(gomock.Any(), chatID, listID).
					Return(database.ShoppingListView{ID: listID, Items: items}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				list, err := reminderService.ShoppingListByID(context.Background(), chatID, listID)
				if list.ID == listID {
					return "loaded list", errorText(err)
				}

				return unexpectedList, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "loaded list"},
		},
		{
			name: "toggle shopping item returns updated repository list",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					ToggleShoppingListItem(gomock.Any(), chatID, itemID).
					Return(database.ShoppingListView{ID: listID, Items: []database.ShoppingListItem{{
						ID:   itemID,
						Text: "Milk",
						Done: true,
					}}}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				list, err := reminderService.ToggleShoppingListItem(context.Background(), chatID, itemID)
				if len(list.Items) == 1 && list.Items[0].Done {
					return "item toggled", errorText(err)
				}

				return "unexpected item", errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "item toggled"},
		},
		{
			name: "create shopping list with empty items is delegated",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateShoppingList(gomock.Any(), chatID, []database.ShoppingListItem{}).
					Return(database.ShoppingListView{ID: listID}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				list, err := reminderService.CreateShoppingList(
					context.Background(),
					chatID,
					[]domain.ShoppingListItem{},
				)
				if list.ID == listID {
					return "empty list", errorText(err)
				}

				return unexpectedList, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "empty list"},
		},
		{
			name: "create shopping list repository error is wrapped",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					CreateShoppingList(gomock.Any(), chatID, items).
					Return(database.ShoppingListView{}, errShoppingRepository).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				list, err := reminderService.CreateShoppingList(context.Background(), chatID, []domain.ShoppingListItem{
					{Text: "Milk", Position: 1},
					{Text: "Bread", Position: 2},
				})
				if list.ID == 0 {
					return "", errorText(err)
				}

				return unexpectedList, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "create shopping list: shopping repository"},
		},
		{
			name: "toggle shopping item repository error is wrapped",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					ToggleShoppingListItem(gomock.Any(), chatID, itemID).
					Return(database.ShoppingListView{}, errShoppingRepository).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, string) {
				list, err := reminderService.ToggleShoppingListItem(context.Background(), chatID, itemID)
				if list.ID == 0 {
					return "", errorText(err)
				}

				return unexpectedList, errorText(err)
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "toggle shopping list item: shopping repository"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			planner := mocks.NewMockReminderCreationPlanner(ctrl)

			test.before(repository)

			reminderService := service.NewReminderService(repository, planner)
			result, errText := test.action(reminderService)

			if test.expected.expectedErrText != "" {
				require.Equal(t, test.expected.expectedErrText, errText)
				require.Equal(t, test.expected.result, result)

				return
			}

			require.Empty(t, errText)
			require.Equal(t, test.expected.result, result)
		})
	}
}

func TestReminderServiceProvidesReadAndSettingsOperations(t *testing.T) {
	t.Parallel()

	chatID := int64(100)
	reminderID := int64(55)
	settings := database.UserSettings{
		TelegramChatID:    chatID,
		Language:          "ru",
		Timezone:          "Europe/Moscow",
		IntrusiveEnabled:  true,
		IntrusiveInterval: 30 * time.Minute,
		UpdatedAt:         time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	}
	errRepository := errors.New("repository")

	tests := []struct {
		name     string
		before   func(repository *mocks.MockReminderCommandRepository)
		action   func(reminderService *service.ReminderService) (string, error)
		expected struct {
			result          string
			expectedErrText string
		}
	}{
		{
			name: "find reminder history returns repository page",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, 2, 4).
					Return([]database.ReminderView{{ID: 1}, {ID: 2}}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				reminders, err := reminderService.FindReminderHistoryByChat(context.Background(), chatID, 2, 4)
				if len(reminders) == 2 {
					return "two reminders", err
				}

				return "unexpected reminders", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "two reminders"},
		},
		{
			name: "reminder stats returns repository stats",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					ReminderStatsByChat(gomock.Any(), chatID).
					Return(database.ReminderStats{Completed: 1, Pending: 2, Skipped: 3}, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				stats, err := reminderService.ReminderStatsByChat(context.Background(), chatID)
				if stats.Completed == 1 && stats.Pending == 2 && stats.Skipped == 3 {
					return "stats loaded", err
				}

				return "unexpected stats", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "stats loaded"},
		},
		{
			name: "user settings returns repository settings",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UserSettings(gomock.Any(), chatID).
					Return(settings, nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				userSettings, err := reminderService.UserSettings(context.Background(), chatID)
				if userSettings.Timezone == settings.Timezone {
					return "settings loaded", err
				}

				return "unexpected settings", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "settings loaded"},
		},
		{
			name: "update user timezone delegates to repository",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateUserTimezone(gomock.Any(), chatID, "Asia/Novosibirsk").
					Return(nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				err := reminderService.UpdateUserTimezone(context.Background(), chatID, "Asia/Novosibirsk")

				return "timezone updated", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "timezone updated"},
		},
		{
			name: "update intrusive settings delegates to repository",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UpdateIntrusiveSettings(gomock.Any(), chatID, false, 45*time.Minute).
					Return(nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				err := reminderService.UpdateIntrusiveSettings(context.Background(), chatID, false, 45*time.Minute)

				return "intrusive updated", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "intrusive updated"},
		},
		{
			name: "acknowledge completed reminder delegates with chat id",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					AcknowledgeReminder(gomock.Any(), chatID, reminderID, database.ReminderStatusCompleted).
					Return(nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				err := reminderService.AcknowledgeReminder(
					context.Background(),
					chatID,
					reminderID,
					domain.ReminderStatusCompleted,
				)

				return "completed acknowledged", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "completed acknowledged"},
		},
		{
			name: "acknowledge canceled reminder delegates with chat id",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					AcknowledgeReminder(gomock.Any(), chatID, reminderID, database.ReminderStatusCanceled).
					Return(nil).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				err := reminderService.AcknowledgeReminder(
					context.Background(),
					chatID,
					reminderID,
					domain.ReminderStatusCanceled,
				)

				return "canceled acknowledged", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{result: "canceled acknowledged"},
		},
		{
			name: "find reminder history repository error is wrapped",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, 2, 4).
					Return(nil, errRepository).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				reminders, err := reminderService.FindReminderHistoryByChat(context.Background(), chatID, 2, 4)
				if reminders == nil {
					return "", err
				}

				return "unexpected reminders", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "find reminder history: repository"},
		},
		{
			name: "user settings repository error is wrapped",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					UserSettings(gomock.Any(), chatID).
					Return(database.UserSettings{}, errRepository).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				userSettings, err := reminderService.UserSettings(context.Background(), chatID)
				if userSettings.TelegramChatID == 0 {
					return "", err
				}

				return "unexpected settings", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "load user settings: repository"},
		},
		{
			name: "unsupported acknowledge status returns validation error",
			before: func(_ *mocks.MockReminderCommandRepository) {
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				err := reminderService.AcknowledgeReminder(
					context.Background(),
					chatID,
					reminderID,
					domain.ReminderStatusSkipped,
				)

				return "", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "acknowledge reminder: unsupported reminder status"},
		},
		{
			name: "acknowledge repository error is wrapped",
			before: func(repository *mocks.MockReminderCommandRepository) {
				repository.EXPECT().
					AcknowledgeReminder(gomock.Any(), chatID, reminderID, database.ReminderStatusCompleted).
					Return(errRepository).
					Times(1)
			},
			action: func(reminderService *service.ReminderService) (string, error) {
				err := reminderService.AcknowledgeReminder(
					context.Background(),
					chatID,
					reminderID,
					domain.ReminderStatusCompleted,
				)

				return "", err
			},
			expected: struct {
				result          string
				expectedErrText string
			}{expectedErrText: "acknowledge reminder: repository"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repository := mocks.NewMockReminderCommandRepository(ctrl)
			planner := mocks.NewMockReminderCreationPlanner(ctrl)

			test.before(repository)

			reminderService := service.NewReminderService(repository, planner)
			result, err := test.action(reminderService)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected error")
				require.Equal(t, test.expected.result, result)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.result, result)
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
			reminder        domain.ReminderView
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 1, Title: "Work standup"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 2, Title: "Training"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 3, Title: "Weekend plan"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 4, Title: "Review"}},
		},
		{
			name:         "empty title returns validation error",
			title:        " ",
			desc:         "Weekly",
			weekdaysMask: 64,
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder        domain.ReminderView
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
				reminder        domain.ReminderView
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
			reminder        domain.ReminderView
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 10, Title: "Medicine"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 11, Title: "Water plants"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 12, Title: "Report"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 13, Title: "Pay rent", Description: "Card"}},
		},
		{
			name:         "empty interval title returns validation error",
			title:        " ",
			desc:         "Morning",
			intervalDays: 1,
			before: func(_ *mocks.MockReminderCreationPlanner) {
			},
			expected: struct {
				reminder        domain.ReminderView
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
				reminder        domain.ReminderView
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
			reminder        domain.ReminderView
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 20, Title: "Trip"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 21, Title: "Doctor"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 22, Title: "Tax", Description: "Forms"}},
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
				reminder        domain.ReminderView
				expectedErrText string
			}{reminder: domain.ReminderView{ID: 23, Title: "Tickets"}},
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
				reminder        domain.ReminderView
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
