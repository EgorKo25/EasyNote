//nolint:lll,testpackage // These tests cover internal Telegram UI state transitions without adding production hooks.
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	telegrammocks "github.com/EgorKo25/EasyNote/src/telegram/mocks"
	telegramui "github.com/EgorKo25/EasyNote/src/telegram/ui"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
	telegoapimock "github.com/mymmrac/telego/telegoapi/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errTelegramAPI = errors.New("telegram api")
var testTelegramToken = strings.Join([]string{"1234567890", strings.Repeat("a", 35)}, ":")

var (
	errExpectedSendCaller = errors.New(
		"send telegram reminder 5: telego: sendMessage: internal execution: request call: telegram api",
	)
	errExpectedSendResponse   = errors.New("send telegram reminder 6: telego: sendMessage: api: 400 \"bad request\"")
	errExpectedEmptyTokenEnvE = errors.New("telegram token is empty: env EASY_NOTE_TEST_TOKEN_E")
	errExpectedEmptyTokenEnvF = errors.New("telegram token is empty: env EASY_NOTE_TEST_TOKEN_F")
	errLoadReminders          = errors.New("load reminders")
	errCreateDraftReminder    = errors.New("create draft reminder")
	errTimezoneCity           = errors.New("timezone city")
	errCompleteDelivered      = errors.New("complete delivered")
	errDisableIntrusive       = errors.New("disable intrusive")
	errSettings               = errors.New("settings")
)

func testTelegramResponse(t *testing.T, value any) *telegoapi.Response {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return &telegoapi.Response{Ok: true, Result: data}
}

func TestReminderDraftAppliesTextInput(t *testing.T) {
	t.Parallel()

	chatID := int64(100)

	tests := []struct {
		name     string
		draft    reminderDraft
		input    string
		expected struct {
			applied     bool
			title       string
			description string
			hour        int
			minute      int
			waitingFor  telegramui.Field
			screen      telegramui.Screen
			notice      string
		}
	}{
		{
			name:  "title is saved from user message",
			draft: reminderDraft{waitingFor: telegramui.FieldTitle},
			input: "Pay rent",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: true, title: "Pay rent", screen: telegramui.ScreenMain},
		},
		{
			name:  "description is saved from user message",
			draft: reminderDraft{waitingFor: telegramui.FieldDescription},
			input: "Before lunch",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: true, description: "Before lunch", screen: telegramui.ScreenMain},
		},
		{
			name:  "short title is saved from user message",
			draft: reminderDraft{waitingFor: telegramui.FieldTitle},
			input: "Call",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: true, title: "Call", screen: telegramui.ScreenMain},
		},
		{
			name:  "long description is saved from user message",
			draft: reminderDraft{waitingFor: telegramui.FieldDescription},
			input: "Prepare documents before lunch",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: true, description: "Prepare documents before lunch", screen: telegramui.ScreenMain},
		},
		{
			name: "manual time input is saved from HH MM message",
			draft: reminderDraft{
				date:   time.Date(2099, 6, 2, 0, 0, 0, 0, time.Local),
				screen: telegramui.ScreenTime,
			},
			input: "18:45",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: true, hour: 18, minute: 45, screen: telegramui.ScreenMain},
		},
		{
			name:  "message is ignored when draft does not wait for text",
			draft: reminderDraft{screen: telegramui.ScreenMain},
			input: "ignored",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: false, screen: telegramui.ScreenMain},
		},
		{
			name: "invalid manual time input is ignored on time screen",
			draft: reminderDraft{
				date:   time.Date(2099, 6, 2, 0, 0, 0, 0, time.Local),
				screen: telegramui.ScreenTime,
			},
			input: "18-45",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: false, screen: telegramui.ScreenTime},
		},
		{
			name:  "message is ignored when draft does not exist",
			input: "ignored",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  telegramui.Field
				screen      telegramui.Screen
				notice      string
			}{applied: false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			drafts := map[int64]reminderDraft{}
			if test.draft.waitingFor != "" || test.draft.screen != "" {
				drafts[chatID] = test.draft
			}

			bot := &ReminderBot{draftByChat: drafts}

			applied := bot.applyDraftText(t.Context(), chatID, test.input)
			draft := bot.draft(chatID)

			require.Equal(t, test.expected.applied, applied)
			require.Equal(t, test.expected.title, draft.title)
			require.Equal(t, test.expected.description, draft.description)
			require.Equal(t, test.expected.hour, draft.hour)
			require.Equal(t, test.expected.minute, draft.minute)
			require.Equal(t, test.expected.waitingFor, draft.waitingFor)
			require.Equal(t, test.expected.screen, draft.screen)
			require.Equal(t, test.expected.notice, draft.notice)
		})
	}
}

func TestReminderDraftAppliesDateCallbacks(t *testing.T) {
	t.Parallel()

	chatID := int64(200)
	location := time.Local

	tests := []struct {
		name     string
		draft    reminderDraft
		data     string
		expected struct {
			applied bool
			date    time.Time
			hour    int
			minute  int
			screen  telegramui.Screen
		}
	}{
		{
			name:  "date callback selects date and opens time editor",
			draft: reminderDraft{},
			data:  "date:2099-06-04",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  telegramui.Screen
			}{applied: true, date: time.Date(2099, 6, 4, 0, 0, 0, 0, location), screen: telegramui.ScreenTime},
		},
		{
			name:  "month callback changes calendar month",
			draft: reminderDraft{},
			data:  "month:2026-07",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  telegramui.Screen
			}{applied: true, screen: telegramui.ScreenCalendar},
		},
		{
			name:  "time preset callback changes selected time",
			draft: reminderDraft{date: time.Date(2026, 6, 2, 0, 0, 0, 0, location), screen: telegramui.ScreenTime},
			data:  "time:set:18:00",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  telegramui.Screen
			}{applied: true, date: time.Date(2026, 6, 2, 0, 0, 0, 0, location), hour: 18, screen: telegramui.ScreenMain},
		},
		{
			name: "time shift callback moves selected time",
			draft: reminderDraft{
				date:   time.Date(2026, 6, 2, 0, 0, 0, 0, location),
				hour:   10,
				minute: 5,
				screen: telegramui.ScreenTime,
			},
			data: "time:shift:-5",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  telegramui.Screen
			}{applied: true, date: time.Date(2026, 6, 2, 0, 0, 0, 0, location), hour: 10, screen: telegramui.ScreenTime},
		},
		{
			name:  "invalid date callback is rejected",
			draft: reminderDraft{},
			data:  "date:not-a-date",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  telegramui.Screen
			}{applied: false},
		},
		{
			name:  "unknown callback is rejected",
			draft: reminderDraft{},
			data:  "unknown:value",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  telegramui.Screen
			}{applied: false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			bot := &ReminderBot{draftByChat: map[int64]reminderDraft{chatID: test.draft}}

			applied := bot.applyDraftDateCallback(chatID, test.data)
			draft := bot.draft(chatID)

			require.Equal(t, test.expected.applied, applied)
			require.Equal(t, test.expected.date, draft.date)
			require.Equal(t, test.expected.hour, draft.hour)
			require.Equal(t, test.expected.minute, draft.minute)
			require.Equal(t, test.expected.screen, draft.screen)
		})
	}
}

func TestReminderDraftAppliesWeekdayCallbacks(t *testing.T) {
	t.Parallel()

	chatID := int64(900)

	tests := []struct {
		name     string
		draft    reminderDraft
		data     string
		expected struct {
			applied      bool
			repeat       telegramui.Repeat
			weekdaysMask int
			screen       telegramui.Screen
		}
	}{
		{
			name:  "monday callback selects monday for custom weekdays",
			draft: reminderDraft{},
			data:  "weekday:1",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				weekdaysMask int
				screen       telegramui.Screen
			}{applied: true, repeat: telegramui.RepeatCustomWeekdays, weekdaysMask: 1, screen: telegramui.ScreenWeekdays},
		},
		{
			name: "thursday callback adds thursday to existing monday",
			draft: reminderDraft{
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 1,
				screen:       telegramui.ScreenWeekdays,
			},
			data: "weekday:8",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				weekdaysMask int
				screen       telegramui.Screen
			}{
				applied:      true,
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 1 | 8,
				screen:       telegramui.ScreenWeekdays,
			},
		},
		{
			name: "selected weekday callback removes weekday from mask",
			draft: reminderDraft{
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 1 | 8,
				screen:       telegramui.ScreenWeekdays,
			},
			data: "weekday:8",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				weekdaysMask int
				screen       telegramui.Screen
			}{
				applied:      true,
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 1,
				screen:       telegramui.ScreenWeekdays,
			},
		},
		{
			name: "sunday callback preserves existing weekend mask",
			draft: reminderDraft{
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 32,
				screen:       telegramui.ScreenWeekdays,
			},
			data: "weekday:64",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				weekdaysMask int
				screen       telegramui.Screen
			}{
				applied:      true,
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 32 | 64,
				screen:       telegramui.ScreenWeekdays,
			},
		},
		{
			name:  "invalid weekday callback is rejected",
			draft: reminderDraft{},
			data:  "weekday:not-a-number",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				weekdaysMask int
				screen       telegramui.Screen
			}{applied: false},
		},
		{
			name:  "unknown callback is rejected",
			draft: reminderDraft{},
			data:  "weekday-unknown:1",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				weekdaysMask int
				screen       telegramui.Screen
			}{applied: false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			bot := &ReminderBot{draftByChat: map[int64]reminderDraft{chatID: test.draft}}

			applied := bot.applyDraftDateCallback(chatID, test.data)
			draft := bot.draft(chatID)

			require.Equal(t, test.expected.applied, applied)
			require.Equal(t, test.expected.repeat, draft.repeat)
			require.Equal(t, test.expected.weekdaysMask, draft.weekdaysMask)
			require.Equal(t, test.expected.screen, draft.screen)
		})
	}
}
func TestReminderUITextsAndKeyboards(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("MSK", 3*60*60)
	reminder := database.ReminderView{
		ID:          10,
		Title:       "Pay rent",
		Description: "Before lunch",
		RemindAt:    time.Date(2026, 6, 2, 6, 30, 0, 0, time.UTC),
	}

	tests := []struct {
		name     string
		action   func() string
		expected struct {
			expectedText string
		}
	}{
		{
			name:   "main menu text is visually empty",
			action: telegramui.MainMenuText,
			expected: struct {
				expectedText string
			}{
				expectedText: " ",
			},
		},
		{
			name: "draft text shows selected date time and empty repeat",
			action: func() string {
				return telegramui.DraftText(uiDraft(reminderDraft{
					title:       "Pay rent",
					description: "Before lunch",
					date:        time.Date(2026, 6, 2, 0, 0, 0, 0, location),
					hour:        9,
					minute:      30,
				}), location)
			},
			expected: struct {
				expectedText string
			}{
				expectedText: "Создание напоминания\n\n" +
					"Дата и время: 02.06.2026 09:30\n" +
					"Название: Pay rent\n" +
					"Описание: Before lunch\n" +
					"Повторять: нет\n\n" +
					"Выберите дату и время кнопкой ниже.",
			},
		},
		{
			name: "draft text shows selected custom date count",
			action: func() string {
				return telegramui.DraftText(uiDraft(reminderDraft{
					title:       "Pay rent",
					description: "Before lunch",
					repeat:      telegramui.RepeatCustomDates,
					customDates: []time.Time{
						time.Date(2026, 6, 2, 0, 0, 0, 0, location),
						time.Date(2026, 6, 4, 0, 0, 0, 0, location),
					},
				}), location)
			},
			expected: struct {
				expectedText string
			}{
				expectedText: "Создание напоминания\n\n" +
					"Дата и время: не задано\n" +
					"Название: Pay rent\n" +
					"Описание: Before lunch\n" +
					"Повторять: выбрано дат: 2\n\n" +
					"Выберите дату и время кнопкой ниже.",
			},
		},
		{
			name: "reminder view text includes description",
			action: func() string {
				return telegramui.ReminderViewText(reminder, location)
			},
			expected: struct {
				expectedText string
			}{expectedText: "02.06.2026 09:30\nPay rent\nBefore lunch"},
		},
		{
			name: "empty draft text shows missing fields and empty repeat",
			action: func() string {
				return telegramui.DraftText(uiDraft(reminderDraft{}), location)
			},
			expected: struct {
				expectedText string
			}{
				expectedText: "Создание напоминания\n\n" +
					"Дата и время: не задано\n" +
					"Название: не задано\n" +
					"Описание: не задано\n" +
					"Повторять: нет\n\n" +
					"Выберите дату и время кнопкой ниже.",
			},
		},
		{
			name: "empty reminder list explains next action",
			action: func() string {
				return telegramui.ReminderListText(nil)
			},
			expected: struct {
				expectedText string
			}{
				expectedText: "Активных напоминаний пока нет.\n\n" +
					"Создайте первое напоминание на основной клавиатуре.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expected.expectedText, test.action())
		})
	}
}
func TestReminderUIBuildsExpectedKeyboards(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 8, 30, 0, 0, time.Local)
	tests := []struct {
		name     string
		action   func() any
		expected struct {
			rowCount        int
			firstButtonText string
		}
	}{
		{
			name: "main draft keyboard has three command rows",
			action: func() any {
				return telegramui.DraftKeyboard(uiDraft(reminderDraft{}), now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Дата"},
		},
		{
			name: "filled one time draft keyboard offers changing date",
			action: func() any {
				return telegramui.DraftKeyboard(uiDraft(reminderDraft{
					date: time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local),
				}), now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Изменить дату"},
		},
		{
			name: "empty recurring draft keyboard explains start date time",
			action: func() any {
				return telegramui.DraftKeyboard(uiDraft(reminderDraft{repeat: telegramui.RepeatEveryDay}), now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Дата и время начала"},
		},
		{
			name: "filled recurring draft keyboard offers changing start date time",
			action: func() any {
				return telegramui.DraftKeyboard(uiDraft(reminderDraft{
					date:   time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local),
					repeat: telegramui.RepeatEveryDay,
				}), now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Изменить дату и время начала"},
		},
		{
			name: "date choice keyboard is compact",
			action: func() any {
				return telegramui.DraftKeyboard(uiDraft(reminderDraft{screen: telegramui.ScreenDate}), now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 3},
		},
		{
			name: "calendar keyboard opens explicit calendar grid",
			action: func() any {
				return telegramui.DraftKeyboard(uiDraft(reminderDraft{
					screen:        telegramui.ScreenCalendar,
					calendarMonth: time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local),
				}), now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 8},
		},
		{
			name: "time keyboard has future presets, steppers and done",
			action: func() any {
				return telegramui.DraftKeyboard(uiDraft(reminderDraft{
					screen: telegramui.ScreenTime,
					date:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local),
					hour:   9, minute: 30,
				}), now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4},
		},
		{
			name: "empty reminder list keyboard still has pagination and back",
			action: func() any {
				return telegramui.ReminderListKeyboard(nil)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 2, firstButtonText: "<-"},
		},
		{
			name: "reminder list keyboard opens reminders by title buttons",
			action: func() any {
				return telegramui.ReminderListKeyboard([]database.ReminderView{
					{ID: 1, Title: "Pay rent"},
					{ID: 2, Title: "Call mom"},
				})
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "#1 Pay rent"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var rows [][]string

			switch keyboard := test.action().(type) {
			case *telego.InlineKeyboardMarkup:
				for _, row := range keyboard.InlineKeyboard {
					textRow := make([]string, 0, len(row))

					for _, button := range row {
						textRow = append(textRow, button.Text)
					}

					rows = append(rows, textRow)
				}
			case *telego.ReplyKeyboardMarkup:
				for _, row := range keyboard.Keyboard {
					textRow := make([]string, 0, len(row))

					for _, button := range row {
						textRow = append(textRow, button.Text)
					}

					rows = append(rows, textRow)
				}
			}

			require.Len(t, rows, test.expected.rowCount)

			if test.expected.firstButtonText != "" {
				require.Equal(t, test.expected.firstButtonText, rows[0][0])
			}
		})
	}
}

func TestSenderSendsReminder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      config.Telegram
		reminder database.ScheduledReminder
		before   func(caller *telegoapimock.MockCaller)
		expected struct {
			expectedErr error
		}
	}{
		{
			name: "title only reminder is sent",
			reminder: database.ScheduledReminder{
				ID:             1,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 1}), nil).
					Times(1)
			},
		},
		{
			name: "description is appended to reminder text",
			reminder: database.ScheduledReminder{
				ID:             2,
				TelegramChatID: 100,
				Title:          "Pay rent",
				Description:    "Before lunch",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 2}), nil).
					Times(1)
			},
		},
		{
			name: "parse mode reminder is sent",
			cfg:  config.Telegram{ParseMode: telego.ModeHTML},
			reminder: database.ScheduledReminder{
				ID:             3,
				TelegramChatID: 100,
				Title:          "<b>Pay rent</b>",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 3}), nil).
					Times(1)
			},
		},
		{
			name: "silent protected reminder is sent",
			cfg:  config.Telegram{DisableNotification: true, ProtectContent: true},
			reminder: database.ScheduledReminder{
				ID:             4,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 4}), nil).
					Times(1)
			},
		},
		{
			name: "api caller error is returned",
			reminder: database.ScheduledReminder{
				ID:             5,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errTelegramAPI).
					Times(1)
			},
			expected: struct{ expectedErr error }{
				expectedErr: errExpectedSendCaller,
			},
		},
		{
			name: "api response error is returned",
			reminder: database.ScheduledReminder{
				ID:             6,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&telegoapi.Response{
						Ok:    false,
						Error: &telegoapi.Error{ErrorCode: 400, Description: "bad request"},
					}, nil).
					Times(1)
			},
			expected: struct{ expectedErr error }{
				expectedErr: errExpectedSendResponse,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			caller := telegoapimock.NewMockCaller(ctrl)
			test.before(caller)

			bot, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			sender := &Sender{bot: bot, cfg: test.cfg}
			_, err = sender.SendReminder(t.Context(), test.reminder)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestNewSenderValidatesToken(t *testing.T) {
	tests := []struct {
		name     string
		tokenEnv string
		token    string
		expected struct {
			expectedErr error
		}
	}{
		{
			name:     "token from environment creates sender",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_A",
			token:    testTelegramToken,
		},
		{
			name:     "warnings can be disabled",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_B",
			token:    testTelegramToken,
		},
		{
			name:     "custom api base url is accepted",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_C",
			token:    testTelegramToken,
		},
		{
			name:     "request timeout is accepted",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_D",
			token:    testTelegramToken,
		},
		{
			name:     "missing token returns explicit error",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_E",
			expected: struct{ expectedErr error }{
				expectedErr: errExpectedEmptyTokenEnvE,
			},
		},
		{
			name:     "empty token returns explicit error",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_F",
			token:    "",
			expected: struct{ expectedErr error }{
				expectedErr: errExpectedEmptyTokenEnvF,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.tokenEnv, test.token)

			sender, err := NewSender(context.Background(), config.Telegram{
				TokenEnv:         test.tokenEnv,
				APIBaseURL:       "https://api.telegram.org",
				RequestTimeout:   time.Second,
				LongPollingRetry: time.Second,
			})

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())
				require.Nil(t, sender)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, sender)
		})
	}
}

func TestReminderUIBuildsDraftModel(t *testing.T) {
	t.Parallel()

	location := time.Local
	customDate := time.Date(2026, 6, 4, 0, 0, 0, 0, location)

	tests := []struct {
		name     string
		draft    reminderDraft
		expected struct {
			title         string
			description   string
			notice        string
			date          time.Time
			hour          int
			minute        int
			repeat        telegramui.Repeat
			customDates   []time.Time
			weekdaysMask  int
			screen        telegramui.Screen
			calendarMonth time.Time
			waitingFor    telegramui.Field
		}
	}{
		{
			name:  "empty draft maps zero values",
			draft: reminderDraft{},
			expected: struct {
				title         string
				description   string
				notice        string
				date          time.Time
				hour          int
				minute        int
				repeat        telegramui.Repeat
				customDates   []time.Time
				weekdaysMask  int
				screen        telegramui.Screen
				calendarMonth time.Time
				waitingFor    telegramui.Field
			}{},
		},
		{
			name: "one-time draft maps text and date",
			draft: reminderDraft{
				title:       "Pay rent",
				description: "Before lunch",
				date:        time.Date(2026, 6, 2, 0, 0, 0, 0, location),
				hour:        9,
				minute:      30,
				screen:      telegramui.ScreenMain,
			},
			expected: struct {
				title         string
				description   string
				notice        string
				date          time.Time
				hour          int
				minute        int
				repeat        telegramui.Repeat
				customDates   []time.Time
				weekdaysMask  int
				screen        telegramui.Screen
				calendarMonth time.Time
				waitingFor    telegramui.Field
			}{
				title:       "Pay rent",
				description: "Before lunch",
				date:        time.Date(2026, 6, 2, 0, 0, 0, 0, location),
				hour:        9,
				minute:      30,
				screen:      telegramui.ScreenMain,
			},
		},
		{
			name: "recurring draft maps repeat and weekdays",
			draft: reminderDraft{
				title:        "Training",
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 1 | 8,
				screen:       telegramui.ScreenWeekdays,
			},
			expected: struct {
				title         string
				description   string
				notice        string
				date          time.Time
				hour          int
				minute        int
				repeat        telegramui.Repeat
				customDates   []time.Time
				weekdaysMask  int
				screen        telegramui.Screen
				calendarMonth time.Time
				waitingFor    telegramui.Field
			}{
				title:        "Training",
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 1 | 8,
				screen:       telegramui.ScreenWeekdays,
			},
		},
		{
			name: "custom dates draft maps calendar state",
			draft: reminderDraft{
				repeat:        telegramui.RepeatCustomDates,
				customDates:   []time.Time{customDate},
				calendarMonth: time.Date(2026, 6, 1, 0, 0, 0, 0, location),
				screen:        telegramui.ScreenCalendar,
			},
			expected: struct {
				title         string
				description   string
				notice        string
				date          time.Time
				hour          int
				minute        int
				repeat        telegramui.Repeat
				customDates   []time.Time
				weekdaysMask  int
				screen        telegramui.Screen
				calendarMonth time.Time
				waitingFor    telegramui.Field
			}{
				repeat:        telegramui.RepeatCustomDates,
				customDates:   []time.Time{customDate},
				calendarMonth: time.Date(2026, 6, 1, 0, 0, 0, 0, location),
				screen:        telegramui.ScreenCalendar,
			},
		},
		{
			name: "title input draft maps waiting field",
			draft: reminderDraft{
				waitingFor: telegramui.FieldTitle,
				notice:     "Введите название",
				screen:     telegramui.ScreenMain,
			},
			expected: struct {
				title         string
				description   string
				notice        string
				date          time.Time
				hour          int
				minute        int
				repeat        telegramui.Repeat
				customDates   []time.Time
				weekdaysMask  int
				screen        telegramui.Screen
				calendarMonth time.Time
				waitingFor    telegramui.Field
			}{
				waitingFor: telegramui.FieldTitle,
				notice:     "Введите название",
				screen:     telegramui.ScreenMain,
			},
		},
		{
			name: "description input draft maps waiting field",
			draft: reminderDraft{
				waitingFor: telegramui.FieldDescription,
				notice:     "Введите описание",
				screen:     telegramui.ScreenMain,
			},
			expected: struct {
				title         string
				description   string
				notice        string
				date          time.Time
				hour          int
				minute        int
				repeat        telegramui.Repeat
				customDates   []time.Time
				weekdaysMask  int
				screen        telegramui.Screen
				calendarMonth time.Time
				waitingFor    telegramui.Field
			}{
				waitingFor: telegramui.FieldDescription,
				notice:     "Введите описание",
				screen:     telegramui.ScreenMain,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			draft := uiDraft(test.draft)

			require.Equal(t, test.expected.title, draft.Title)
			require.Equal(t, test.expected.description, draft.Description)
			require.Equal(t, test.expected.notice, draft.Notice)
			require.Equal(t, test.expected.date, draft.Date)
			require.Equal(t, test.expected.hour, draft.Hour)
			require.Equal(t, test.expected.minute, draft.Minute)
			require.Equal(t, test.expected.repeat, draft.Repeat)
			require.Equal(t, test.expected.customDates, draft.CustomDates)
			require.Equal(t, test.expected.weekdaysMask, draft.WeekdaysMask)
			require.Equal(t, test.expected.screen, draft.Screen)
			require.Equal(t, test.expected.calendarMonth, draft.CalendarMonth)
			require.Equal(t, test.expected.waitingFor, draft.WaitingFor)
		})
	}
}

func TestReminderBotHandlesMessages(t *testing.T) {
	t.Parallel()

	chatID := int64(400)
	messageID := 55

	tests := []struct {
		name     string
		message  telego.Message
		draft    reminderDraft
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			expectedErr error
			draftTitle  string
		}
	}{
		{
			name:    "start command opens main menu",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: "/start"},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 1}), nil).
					Times(1)
			},
		},
		{
			name: "create reminder command opens draft",
			message: telego.Message{
				MessageID: messageID,
				Chat:      telego.Chat{ID: chatID},
				Text:      telegramui.ButtonCreateReminder,
			},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 2}), nil).
					Times(2)
			},
		},
		{
			name: "reminders command opens reminder submenu",
			message: telego.Message{
				MessageID: messageID,
				Chat:      telego.Chat{ID: chatID},
				Text:      telegramui.ButtonReminders,
			},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 3}), nil).
					Times(2)
			},
		},
		{
			name:    "draft title message updates current draft",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: "Pay rent"},
			draft:   reminderDraft{waitingFor: telegramui.FieldTitle},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 4}), nil).
					Times(2)
			},
			expected: struct {
				expectedErr error
				draftTitle  string
			}{draftTitle: "Pay rent"},
		},
		{
			name:    "unknown message returns main menu prompt",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: "unknown"},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 5}), nil).
					Times(2)
			},
		},
		{
			name:    "legacy reminder list command returns main menu prompt",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: "Список напоминаний"},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 6}), nil).
					Times(2)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(caller, service, repository)

			botAPI, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			uiByChat := map[int64]int{}
			if test.draft.waitingFor != "" {
				uiByChat[chatID] = messageID
			}

			bot := &ReminderBot{
				bot:         botAPI,
				service:     service,
				repository:  repository,
				draftByChat: map[int64]reminderDraft{chatID: test.draft},
				uiByChat:    uiByChat,
			}

			err = bot.handleMessage(t.Context(), test.message)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.draftTitle, bot.draft(chatID).title)
		})
	}
}

func TestReminderBotRunsReplyMessageCommands(t *testing.T) {
	t.Parallel()

	chatID := int64(410)
	messageID := 77
	reminder := database.ReminderView{ID: 1, Title: "Pay rent", RemindAt: time.Date(2099, 6, 2, 9, 30, 0, 0, time.UTC)}

	tests := []struct {
		name     string
		text     string
		botState func(bot *ReminderBot)
		before   func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			handled         bool
			expectedErrText string
			draftExists     bool
		}
	}{
		{
			name: "create command opens create menu",
			text: telegramui.ButtonCreateReminder,
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: true},
		},
		{
			name: "create reminder kind starts draft",
			text: telegramui.ButtonCreateReminderKind,
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: true, draftExists: true},
		},
		{
			name: "active reminders command loads first database page",
			text: telegramui.ButtonReminderList,
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, reminderPageSize, 0).
					Return([]database.ReminderView{reminder}, nil).
					Times(1)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: true},
		},
		{
			name: "next page command loads shifted database page",
			text: telegramui.ButtonNextPage,
			botState: func(bot *ReminderBot) {
				bot.setReminderListState(chatID, reminderListState{kind: reminderListActive, offset: 2})
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, reminderPageSize, 4).
					Return([]database.ReminderView{reminder}, nil).
					Times(1)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: true},
		},
		{
			name: "settings command loads user settings",
			text: telegramui.ButtonSettings,
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().
					UserSettings(gomock.Any(), chatID).
					Return(database.UserSettings{Timezone: "UTC"}, nil).
					Times(2)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: true},
		},
		{
			name: "draft date command opens date reply keyboard",
			text: telegramui.ButtonDraftDate,
			botState: func(bot *ReminderBot) {
				bot.draftByChat[chatID] = reminderDraft{screen: telegramui.ScreenMain}
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: true, draftExists: true},
		},
		{
			name: "unknown command is not handled",
			text: "unknown",
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: false},
		},
		{
			name: "malformed reminder button returns parse error",
			text: "#bad",
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				handled         bool
				expectedErrText string
				draftExists     bool
			}{handled: true, expectedErrText: `parse reminder id: strconv.ParseInt: parsing "bad": invalid syntax`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(service, repository)

			caller.EXPECT().
				Call(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
				AnyTimes()

			botAPI, err := telego.NewBot(testTelegramToken, telego.WithAPICaller(caller))
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:            botAPI,
				service:        service,
				repository:     repository,
				draftByChat:    make(map[int64]reminderDraft),
				uiByChat:       make(map[int64]int),
				locationByChat: make(map[int64]*time.Location),
				listByChat:     make(map[int64]reminderListState),
				timezoneByChat: make(map[int64]int),
				menuByChat:     make(map[int64]chatMenu),
			}
			if test.botState != nil {
				test.botState(bot)
			}

			err, handled := bot.runKnownMessageCommand(t.Context(), chatID, test.text)

			require.Equal(t, test.expected.handled, handled)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected reply command error")

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.draftExists, bot.draftExists(chatID))
		})
	}
}

func TestReminderBotHandlesReplyNavigationState(t *testing.T) {
	t.Parallel()

	chatID := int64(420)
	messageID := 88
	reminder := database.ReminderView{ID: 7, Title: "Pay rent", RemindAt: time.Date(2099, 6, 2, 9, 30, 0, 0, time.UTC)}

	tests := []struct {
		name     string
		action   func(ctx context.Context, bot *ReminderBot) error
		before   func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		state    func(bot *ReminderBot)
		expected struct {
			expectedErrText string
		}
	}{
		{
			name: "back closes draft sub screen",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleBackMessage(ctx, chatID)
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.draftByChat[chatID] = reminderDraft{screen: telegramui.ScreenDate}
			},
		},
		{
			name: "back from timezone saves selected timezone",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleBackMessage(ctx, chatID)
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UpdateUserTimezone(gomock.Any(), chatID, "Europe/Samara").Return(nil).Times(1),
					repository.EXPECT().
						UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{Timezone: "UTC"}, nil).
						Times(1),
					repository.EXPECT().
						UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{Timezone: "UTC"}, nil).
						Times(1),
				)
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuTimezone
				bot.timezoneByChat[chatID] = 2
			},
		},
		{
			name: "done from intrusive interval returns to intrusive settings",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleDoneMessage(ctx, chatID)
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().UserSettings(gomock.Any(), chatID).
					Return(database.UserSettings{Timezone: "UTC", IntrusiveInterval: time.Minute}, nil).
					Times(2)
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuIntrusiveInterval
			},
		},
		{
			name: "timezone shift updates selected timezone page",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleTimeShiftMessage(ctx, chatID, 60)
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuTimezone
			},
		},
		{
			name: "intrusive interval shift inside boundaries saves interval",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleTimeShiftMessage(ctx, chatID, -5)
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{IntrusiveEnabled: true, IntrusiveInterval: 30 * time.Minute}, nil).
						Times(1),
					repository.EXPECT().
						UpdateIntrusiveSettings(gomock.Any(), chatID, true, 25*time.Minute).
						Return(nil).
						Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{IntrusiveEnabled: true, IntrusiveInterval: 25 * time.Minute}, nil).
						Times(1),
				)
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuIntrusiveInterval
			},
		},
		{
			name: "intrusive interval shift below lower boundary does not save interval",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleTimeShiftMessage(ctx, chatID, -5)
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{IntrusiveEnabled: true, IntrusiveInterval: time.Minute}, nil).
						Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{IntrusiveEnabled: true, IntrusiveInterval: time.Minute}, nil).
						Times(1),
				)
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuIntrusiveInterval
			},
		},
		{
			name: "intrusive interval shift above upper boundary does not save interval",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleTimeShiftMessage(ctx, chatID, 60)
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{IntrusiveEnabled: true, IntrusiveInterval: 2 * time.Hour}, nil).
						Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{IntrusiveEnabled: true, IntrusiveInterval: 2 * time.Hour}, nil).
						Times(1),
				)
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuIntrusiveInterval
			},
		},
		{
			name: "reply reminder button opens current page reminder",
			action: func(ctx context.Context, bot *ReminderBot) error {
				handled, err := bot.handleSelectedReplyMessage(ctx, chatID, telegramui.ReminderButtonText(reminder))
				require.True(t, handled)

				return err
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, reminderPageSize, 0).
					Return([]database.ReminderView{reminder}, nil).
					Times(1)
			},
			state: func(bot *ReminderBot) {
				bot.listByChat[chatID] = reminderListState{kind: reminderListActive}
			},
		},
		{
			name: "reply history reminder button opens current history page reminder",
			action: func(ctx context.Context, bot *ReminderBot) error {
				handled, err := bot.handleSelectedReplyMessage(ctx, chatID, telegramui.ReminderButtonText(reminder))
				require.True(t, handled)

				return err
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().
					FindReminderHistoryByChat(gomock.Any(), chatID, reminderPageSize, 2).
					Return([]database.ReminderView{reminder}, nil).
					Times(1)
			},
			state: func(bot *ReminderBot) {
				bot.listByChat[chatID] = reminderListState{kind: reminderListHistory, offset: 2}
			},
		},
		{
			name: "timezone reply previous button shifts timezone",
			action: func(ctx context.Context, bot *ReminderBot) error {
				handled, err := bot.handleSelectedReplyMessage(ctx, chatID, telegramui.ButtonPreviousPage)
				require.True(t, handled)

				return err
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuTimezone
				bot.timezoneByChat[chatID] = 1
			},
		},
		{
			name: "draft time reply text updates draft time",
			action: func(ctx context.Context, bot *ReminderBot) error {
				handled, err := bot.handleSelectedReplyMessage(ctx, chatID, "18:45")
				require.True(t, handled)

				return err
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.draftByChat[chatID] = reminderDraft{
					date:   time.Date(2099, 6, 2, 0, 0, 0, 0, time.Local),
					screen: telegramui.ScreenTime,
				}
			},
		},
		{
			name: "back from reminder list returns to reminders menu",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleBackMessage(ctx, chatID)
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.menuByChat[chatID] = chatMenuReminderList
			},
		},
		{
			name: "back from main draft returns to create menu",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleBackMessage(ctx, chatID)
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.draftByChat[chatID] = reminderDraft{screen: telegramui.ScreenMain}
			},
		},
		{
			name: "done closes draft time screen",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleDoneMessage(ctx, chatID)
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.draftByChat[chatID] = reminderDraft{screen: telegramui.ScreenTime}
			},
		},
		{
			name: "draft time shift changes selected time",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.handleTimeShiftMessage(ctx, chatID, 5)
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			state: func(bot *ReminderBot) {
				bot.draftByChat[chatID] = reminderDraft{
					date:   time.Date(2099, 6, 2, 0, 0, 0, 0, time.Local),
					hour:   9,
					screen: telegramui.ScreenTime,
				}
			},
		},
		{
			name: "malformed reply reminder button returns parse error",
			action: func(ctx context.Context, bot *ReminderBot) error {
				handled, err := bot.handleSelectedReplyMessage(ctx, chatID, "#bad")
				require.True(t, handled)

				return err
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: `parse reminder id: strconv.ParseInt: parsing "bad": invalid syntax`},
		},
		{
			name: "unknown selected reply message is ignored",
			action: func(ctx context.Context, bot *ReminderBot) error {
				handled, err := bot.handleSelectedReplyMessage(ctx, chatID, "ignored")
				require.False(t, handled)

				return err
			},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(service, repository)

			caller.EXPECT().
				Call(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
				AnyTimes()

			botAPI, err := telego.NewBot(testTelegramToken, telego.WithAPICaller(caller))
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:            botAPI,
				service:        service,
				repository:     repository,
				draftByChat:    make(map[int64]reminderDraft),
				uiByChat:       make(map[int64]int),
				locationByChat: make(map[int64]*time.Location),
				listByChat:     make(map[int64]reminderListState),
				timezoneByChat: make(map[int64]int),
				menuByChat:     make(map[int64]chatMenu),
			}
			if test.state != nil {
				test.state(bot)
			}

			err = test.action(t.Context(), bot)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected reply navigation error")

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestReminderBotCommandCallbacks(t *testing.T) {
	t.Parallel()

	chatID := int64(300)
	messageID := 10

	tests := []struct {
		name     string
		data     string
		draft    reminderDraft
		expected struct {
			handled bool
			screen  telegramui.Screen
			waiting telegramui.Field
		}
	}{
		{
			name:  "title callback waits for title",
			data:  telegramui.CallbackDraftTitle,
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  telegramui.Screen
				waiting telegramui.Field
			}{handled: true, screen: telegramui.ScreenMain, waiting: telegramui.FieldTitle},
		},
		{
			name:  "description callback waits for description",
			data:  telegramui.CallbackDraftDescription,
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  telegramui.Screen
				waiting telegramui.Field
			}{handled: true, screen: telegramui.ScreenMain, waiting: telegramui.FieldDescription},
		},
		{
			name:  "calendar callback opens calendar",
			data:  telegramui.CallbackDraftCalendar,
			draft: reminderDraft{screen: telegramui.ScreenDate},
			expected: struct {
				handled bool
				screen  telegramui.Screen
				waiting telegramui.Field
			}{handled: true, screen: telegramui.ScreenCalendar},
		},
		{
			name:  "date callback opens compact date choice",
			data:  telegramui.CallbackDraftDate,
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  telegramui.Screen
				waiting telegramui.Field
			}{handled: true, screen: telegramui.ScreenDate},
		},
		{
			name:  "unknown callback is not handled",
			data:  "unknown",
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  telegramui.Screen
				waiting telegramui.Field
			}{handled: false},
		},
		{
			name:  "noop callback is not a draft command",
			data:  telegramui.CallbackNoop,
			draft: reminderDraft{screen: telegramui.ScreenMain},
			expected: struct {
				handled bool
				screen  telegramui.Screen
				waiting telegramui.Field
			}{handled: false, screen: telegramui.ScreenMain},
		},
		{
			name:  "back callback returns to main draft",
			data:  telegramui.CallbackDraftBack,
			draft: reminderDraft{screen: telegramui.ScreenCalendar},
			expected: struct {
				handled bool
				screen  telegramui.Screen
				waiting telegramui.Field
			}{handled: true, screen: telegramui.ScreenMain},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			caller := telegoapimock.NewMockCaller(ctrl)
			if test.expected.handled {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(1)
			}

			botAPI, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:         botAPI,
				draftByChat: map[int64]reminderDraft{chatID: test.draft},
				uiByChat:    map[int64]int{chatID: messageID},
			}

			handled, err := bot.handleDraftCommandCallback(t.Context(), chatID, messageID, test.data)
			draft := bot.draft(chatID)

			require.NoError(t, err)
			require.Equal(t, test.expected.handled, handled)
			require.Equal(t, test.expected.screen, draft.screen)
			require.Equal(t, test.expected.waiting, draft.waitingFor)
		})
	}
}

func TestReminderBotAppliesRepeatDateAndIntervalCallbacks(t *testing.T) {
	t.Parallel()

	chatID := int64(310)
	location := time.UTC

	tests := []struct {
		name     string
		draft    reminderDraft
		data     string
		expected struct {
			applied      bool
			repeat       telegramui.Repeat
			screen       telegramui.Screen
			waitingFor   telegramui.Field
			intervalDays int
			customDates  []time.Time
			notice       string
		}
	}{
		{
			name:  "custom dates repeat opens calendar",
			draft: reminderDraft{},
			data:  telegramui.CallbackRepeatPrefix + string(telegramui.RepeatCustomDates),
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{applied: true, repeat: telegramui.RepeatCustomDates, screen: telegramui.ScreenCalendar},
		},
		{
			name:  "custom weekdays repeat opens weekdays screen",
			draft: reminderDraft{},
			data:  telegramui.CallbackRepeatPrefix + string(telegramui.RepeatCustomWeekdays),
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{applied: true, repeat: telegramui.RepeatCustomWeekdays, screen: telegramui.ScreenWeekdays},
		},
		{
			name:  "custom interval repeat opens interval input",
			draft: reminderDraft{},
			data:  telegramui.CallbackRepeatPrefix + string(telegramui.RepeatCustomInterval),
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{
				applied: true, repeat: telegramui.RepeatCustomInterval, screen: telegramui.ScreenInterval,
				waitingFor: telegramui.FieldRepeatInterval, intervalDays: 1,
			},
		},
		{
			name:  "custom date callback adds selected date",
			draft: reminderDraft{repeat: telegramui.RepeatCustomDates},
			data:  telegramui.CallbackDatePrefix + "2099-06-05",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{
				applied: true, repeat: telegramui.RepeatCustomDates, screen: telegramui.ScreenCalendar,
				customDates: []time.Time{time.Date(2099, 6, 5, 0, 0, 0, 0, location)},
			},
		},
		{
			name: "custom date callback removes selected date",
			draft: reminderDraft{
				repeat:      telegramui.RepeatCustomDates,
				customDates: []time.Time{time.Date(2099, 6, 5, 0, 0, 0, 0, location)},
			},
			data: telegramui.CallbackDatePrefix + "2099-06-05",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{
				applied:     true,
				repeat:      telegramui.RepeatCustomDates,
				screen:      telegramui.ScreenCalendar,
				customDates: []time.Time{},
			},
		},
		{
			name:  "interval text saves positive interval",
			draft: reminderDraft{waitingFor: telegramui.FieldRepeatInterval, screen: telegramui.ScreenInterval},
			data:  "5",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{applied: true, repeat: telegramui.RepeatCustomInterval, screen: telegramui.ScreenMain, intervalDays: 5},
		},
		{
			name:  "invalid repeat date is ignored",
			draft: reminderDraft{repeat: telegramui.RepeatCustomDates},
			data:  telegramui.CallbackDatePrefix + "bad-date",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{applied: false, repeat: telegramui.RepeatCustomDates},
		},
		{
			name:  "zero interval text keeps interval screen with notice",
			draft: reminderDraft{waitingFor: telegramui.FieldRepeatInterval, screen: telegramui.ScreenInterval},
			data:  "0",
			expected: struct {
				applied      bool
				repeat       telegramui.Repeat
				screen       telegramui.Screen
				waitingFor   telegramui.Field
				intervalDays int
				customDates  []time.Time
				notice       string
			}{
				applied: true, screen: telegramui.ScreenInterval,
				notice: "Введите интервал в днях: 1, 2, 3 ...",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			bot := &ReminderBot{
				draftByChat: map[int64]reminderDraft{chatID: test.draft},
				locationByChat: map[int64]*time.Location{
					chatID: location,
				},
			}

			var applied bool
			if test.draft.waitingFor == telegramui.FieldRepeatInterval {
				applied = bot.applyDraftText(t.Context(), chatID, test.data)
			} else {
				applied = bot.applyDraftDateCallback(chatID, test.data)
			}

			draft := bot.draft(chatID)

			require.Equal(t, test.expected.applied, applied)
			require.Equal(t, test.expected.repeat, draft.repeat)
			require.Equal(t, test.expected.screen, draft.screen)
			require.Equal(t, test.expected.waitingFor, draft.waitingFor)
			require.Equal(t, test.expected.intervalDays, draft.intervalDays)
			require.Equal(t, test.expected.customDates, draft.customDates)
			require.Equal(t, test.expected.notice, draft.notice)
		})
	}
}

func TestReminderBotAppliesSettingsTextInput(t *testing.T) {
	t.Parallel()

	chatID := int64(315)

	tests := []struct {
		name     string
		draft    reminderDraft
		input    string
		before   func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			applied     bool
			draftExists bool
			notice      string
			waitingFor  telegramui.Field
		}
	}{
		{
			name:  "intrusive interval updates existing enabled setting",
			draft: reminderDraft{waitingFor: telegramui.FieldIntrusiveInterval},
			input: "45",
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UserSettings(gomock.Any(), chatID).Return(database.UserSettings{
						IntrusiveEnabled: true,
					}, nil).Times(1),
					repository.EXPECT().
						UpdateIntrusiveSettings(gomock.Any(), chatID, true, 45*time.Minute).
						Return(nil).
						Times(1),
				)
			},
			expected: struct {
				applied     bool
				draftExists bool
				notice      string
				waitingFor  telegramui.Field
			}{applied: true},
		},
		{
			name:  "intrusive interval updates disabled setting",
			draft: reminderDraft{waitingFor: telegramui.FieldIntrusiveInterval},
			input: "120",
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UserSettings(gomock.Any(), chatID).Return(database.UserSettings{
						IntrusiveEnabled: false,
					}, nil).Times(1),
					repository.EXPECT().
						UpdateIntrusiveSettings(gomock.Any(), chatID, false, 2*time.Hour).
						Return(nil).
						Times(1),
				)
			},
			expected: struct {
				applied     bool
				draftExists bool
				notice      string
				waitingFor  telegramui.Field
			}{applied: true},
		},
		{
			name:  "repeat interval input updates draft through text path",
			draft: reminderDraft{waitingFor: telegramui.FieldRepeatInterval, screen: telegramui.ScreenInterval},
			input: "7",
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				applied     bool
				draftExists bool
				notice      string
				waitingFor  telegramui.Field
			}{applied: true, draftExists: true},
		},
		{
			name:  "second repeat interval input updates draft through text path",
			draft: reminderDraft{waitingFor: telegramui.FieldRepeatInterval, screen: telegramui.ScreenInterval},
			input: "14",
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				applied     bool
				draftExists bool
				notice      string
				waitingFor  telegramui.Field
			}{applied: true, draftExists: true},
		},
		{
			name:  "invalid intrusive interval text keeps draft with notice",
			draft: reminderDraft{waitingFor: telegramui.FieldIntrusiveInterval},
			input: "abc",
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				applied     bool
				draftExists bool
				notice      string
				waitingFor  telegramui.Field
			}{
				applied: true, draftExists: true,
				notice: "Введите интервал в минутах от 1 до 120.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(service, repository)

			bot := &ReminderBot{
				service:        service,
				repository:     repository,
				draftByChat:    map[int64]reminderDraft{chatID: test.draft},
				locationByChat: make(map[int64]*time.Location),
			}

			applied := bot.applyDraftText(t.Context(), chatID, test.input)
			_, exists := bot.draftByChat[chatID]
			draft := bot.draft(chatID)

			require.Equal(t, test.expected.applied, applied)
			require.Equal(t, test.expected.draftExists, exists)
			require.Equal(t, test.expected.notice, draft.notice)
			require.Equal(t, test.expected.waitingFor, draft.waitingFor)
		})
	}
}

func TestReminderBotHandlesReminderCallbacks(t *testing.T) {
	t.Parallel()

	chatID := int64(320)
	messageID := 15
	reminder := database.ReminderView{
		ID:       10,
		Title:    "Pay rent",
		RemindAt: time.Date(2099, 6, 2, 9, 30, 0, 0, time.Local),
	}

	tests := []struct {
		name     string
		data     string
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			expectedErrText string
			draftExists     bool
		}
	}{
		{
			name: "create reminder callback starts draft in current message",
			data: telegramui.CallbackCreateReminder,
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(1)
			},
			expected: struct {
				expectedErrText string
				draftExists     bool
			}{draftExists: true},
		},
		{
			name: "reminder list callback edits reminder list",
			data: telegramui.CallbackReminderList,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
						Return([]database.ReminderView{reminder}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(2),
				)
			},
		},
		{
			name: "view reminder callback edits reminder details",
			data: telegramui.CallbackViewPrefix + "10",
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
						Return([]database.ReminderView{reminder}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name: "missing viewed reminder returns list view",
			data: telegramui.CallbackViewPrefix + "99",
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
						Return([]database.ReminderView{reminder}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(2),
				)
			},
		},
		{
			name: "delete reminder callback deletes reminder and edits list",
			data: telegramui.CallbackDeletePrefix + "10",
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().DeleteReminder(gomock.Any(), chatID, int64(10)).
						Return(nil).
						Times(1),
					repository.EXPECT().FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
						Return([]database.ReminderView{}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(2),
				)
			},
		},
		{
			name: "invalid view callback returns parse error",
			data: telegramui.CallbackViewPrefix + "bad",
			before: func(_ *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				expectedErrText string
				draftExists     bool
			}{expectedErrText: `parse reminder id: strconv.ParseInt: parsing "bad": invalid syntax`},
		},
		{
			name: "view reminder service error is returned",
			data: telegramui.CallbackViewPrefix + "10",
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
					Return(nil, errLoadReminders).
					Times(1)
			},
			expected: struct {
				expectedErrText string
				draftExists     bool
			}{expectedErrText: "load reminders: load reminders"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(caller, service, repository)

			botAPI, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:         botAPI,
				service:     service,
				repository:  repository,
				draftByChat: map[int64]reminderDraft{},
				uiByChat:    map[int64]int{chatID: messageID},
			}

			err = bot.handleReminderCallback(t.Context(), chatID, messageID, test.data)

			if test.expected.expectedErrText != "" {
				require.EqualError(t, err, test.expected.expectedErrText)

				return
			}

			require.NoError(t, err)

			_, exists := bot.draftByChat[chatID]
			require.Equal(t, test.expected.draftExists, exists)
		})
	}
}

func TestReminderBotAcknowledgesDeliveredReminders(t *testing.T) {
	t.Parallel()

	chatID := int64(330)
	messageID := 25

	tests := []struct {
		name     string
		data     string
		prefix   string
		status   database.ReminderStatus
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			expectedErrText string
		}
	}{
		{
			name:   "completed delivered reminder is acknowledged",
			data:   telegramui.CallbackReminderDone + "10",
			prefix: telegramui.CallbackReminderDone,
			status: database.ReminderStatusCompleted,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().CompleteOccurrence(
						gomock.Any(),
						int64(10),
						database.ReminderStatusCompleted,
						gomock.Any(),
					).Return(nil).Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:   "canceled delivered reminder is acknowledged",
			data:   telegramui.CallbackReminderCancelDelivered + "11",
			prefix: telegramui.CallbackReminderCancelDelivered,
			status: database.ReminderStatusCanceled,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().CompleteOccurrence(
						gomock.Any(),
						int64(11),
						database.ReminderStatusCanceled,
						gomock.Any(),
					).Return(nil).Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:   "second completed delivered reminder is acknowledged",
			data:   telegramui.CallbackReminderDone + "12",
			prefix: telegramui.CallbackReminderDone,
			status: database.ReminderStatusCompleted,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().CompleteOccurrence(
						gomock.Any(),
						int64(12),
						database.ReminderStatusCompleted,
						gomock.Any(),
					).Return(nil).Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:   "second canceled delivered reminder is acknowledged",
			data:   telegramui.CallbackReminderCancelDelivered + "13",
			prefix: telegramui.CallbackReminderCancelDelivered,
			status: database.ReminderStatusCanceled,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().CompleteOccurrence(
						gomock.Any(),
						int64(13),
						database.ReminderStatusCanceled,
						gomock.Any(),
					).Return(nil).Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:   "invalid delivered reminder id returns parse error",
			data:   telegramui.CallbackReminderDone + "bad",
			prefix: telegramui.CallbackReminderDone,
			status: database.ReminderStatusCompleted,
			before: func(_ *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: `parse delivered reminder id: strconv.ParseInt: parsing "bad": invalid syntax`},
		},
		{
			name:   "complete delivered reminder service error is returned",
			data:   telegramui.CallbackReminderDone + "14",
			prefix: telegramui.CallbackReminderDone,
			status: database.ReminderStatusCompleted,
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				service.EXPECT().CompleteOccurrence(
					gomock.Any(),
					int64(14),
					database.ReminderStatusCompleted,
					gomock.Any(),
				).
					Return(errCompleteDelivered).
					Times(1)
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: "acknowledge delivered reminder: complete delivered"},
		},
		{
			name:   "unsupported delivered reminder status is returned",
			data:   telegramui.CallbackReminderDone + "15",
			prefix: telegramui.CallbackReminderDone,
			status: database.ReminderStatusSent,
			before: func(_ *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: "acknowledge delivered reminder: unsupported delivered reminder status: sent"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(caller, service, repository)

			botAPI, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:        botAPI,
				service:    service,
				repository: repository,
				uiByChat:   map[int64]int{chatID: messageID},
			}

			err = bot.acknowledgeReminder(t.Context(), chatID, messageID, test.data, test.prefix, test.status)

			if test.expected.expectedErrText != "" {
				require.EqualError(t, err, test.expected.expectedErrText)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestReminderBotHandlesSettingsCallbacks(t *testing.T) {
	t.Parallel()

	chatID := int64(340)
	messageID := 35

	tests := []struct {
		name     string
		data     string
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			handled         bool
			expectedErrText string
			waitingFor      telegramui.Field
		}
	}{
		{
			name: "language callback edits language message",
			data: telegramui.CallbackSettingsLanguage,
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(2)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true},
		},
		{
			name: "timezone callback opens timezone keyboard",
			data: telegramui.CallbackSettingsTimezone,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{Timezone: "Europe/Moscow"}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(2),
				)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true},
		},
		{
			name: "timezone next callback edits timezone keyboard",
			data: telegramui.TimezoneNextCallback(0),
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(2)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true},
		},
		{
			name: "timezone prev callback edits timezone keyboard",
			data: telegramui.TimezonePrevCallback(1),
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(2)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true},
		},
		{
			name: "timezone select callback saves timezone",
			data: telegramui.TimezoneSelectCallback(0),
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UpdateUserTimezone(gomock.Any(), chatID, "Europe/Moscow").Return(nil).Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{Timezone: "Europe/Moscow"}, nil).
						Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{Timezone: "Europe/Moscow"}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(2),
				)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true},
		},
		{
			name: "timezone callback returns settings load error",
			data: telegramui.CallbackSettingsTimezone,
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().UserSettings(gomock.Any(), chatID).
					Return(database.UserSettings{}, errSettings).
					Times(1)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true, expectedErrText: "load user settings: settings"},
		},
		{
			name: "timezone select callback returns service error",
			data: telegramui.TimezoneSelectCallback(0),
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().UpdateUserTimezone(gomock.Any(), chatID, "Europe/Moscow").
					Return(errTimezoneCity).
					Times(1)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true, expectedErrText: "set user timezone: timezone city"},
		},
		{
			name: "intrusive callback waits for interval and edits settings",
			data: telegramui.CallbackSettingsIntrusive,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().UserSettings(gomock.Any(), chatID).Return(database.UserSettings{
						Timezone:          "Europe/Moscow",
						IntrusiveInterval: 30 * time.Minute,
					}, nil).Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).Return(database.UserSettings{
						Timezone:          "Europe/Moscow",
						IntrusiveInterval: 30 * time.Minute,
					}, nil).Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(2),
				)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true, waitingFor: telegramui.FieldIntrusiveInterval},
		},
		{
			name: "intrusive on callback enables intrusive reminders",
			data: telegramui.CallbackSettingsIntrusiveOn,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().
						UpdateIntrusiveSettings(gomock.Any(), chatID, true, 30*time.Minute).
						Return(nil).
						Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).Return(database.UserSettings{
						Timezone:          "Europe/Moscow",
						IntrusiveEnabled:  true,
						IntrusiveInterval: 30 * time.Minute,
					}, nil).Times(1),
					repository.EXPECT().UserSettings(gomock.Any(), chatID).Return(database.UserSettings{
						Timezone:          "Europe/Moscow",
						IntrusiveEnabled:  true,
						IntrusiveInterval: 30 * time.Minute,
					}, nil).Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(2),
				)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true},
		},
		{
			name: "unknown settings callback is not handled",
			data: "settings:unknown",
			before: func(_ *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: false},
		},
		{
			name: "intrusive off callback returns service error",
			data: telegramui.CallbackSettingsIntrusiveOff,
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().
					UpdateIntrusiveSettings(gomock.Any(), chatID, false, 30*time.Minute).
					Return(errDisableIntrusive).
					Times(1)
			},
			expected: struct {
				handled         bool
				expectedErrText string
				waitingFor      telegramui.Field
			}{handled: true, expectedErrText: "disable intrusive reminders: disable intrusive"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(caller, service, repository)

			botAPI, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:            botAPI,
				service:        service,
				repository:     repository,
				draftByChat:    map[int64]reminderDraft{chatID: {}},
				uiByChat:       map[int64]int{chatID: messageID},
				locationByChat: make(map[int64]*time.Location),
			}

			handled, err := bot.handleSettingsCallback(t.Context(), chatID, messageID, test.data)
			draft := bot.draft(chatID)

			if test.expected.expectedErrText != "" {
				require.EqualError(t, err, test.expected.expectedErrText)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, test.expected.handled, handled)
			require.Equal(t, test.expected.waitingFor, draft.waitingFor)
		})
	}
}

func TestReminderBotSendsListsAndSettingsScreens(t *testing.T) {
	t.Parallel()

	chatID := int64(345)
	reminder := database.ReminderView{
		ID:       21,
		Title:    "Pay rent",
		RemindAt: time.Date(2099, 6, 2, 9, 30, 0, 0, time.UTC),
		Status:   database.ReminderStatusCompleted,
	}

	tests := []struct {
		name     string
		action   func(context.Context, *ReminderBot) error
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			expectedErrText string
		}
	}{
		{
			name: "active reminder list is sent",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.sendReminderList(ctx, chatID)
			},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().
						FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
						Return([]database.ReminderView{reminder}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: 41}), nil).
						Times(1),
				)
			},
		},
		{
			name: "empty active reminder list is sent",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.sendReminderList(ctx, chatID)
			},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().
						FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
						Return([]database.ReminderView{}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: 42}), nil).
						Times(1),
				)
			},
		},
		{
			name: "reminder history is sent",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.sendReminderHistory(ctx, chatID)
			},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().
						UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{Timezone: "UTC"}, nil).
						Times(1),
					repository.EXPECT().
						FindReminderHistoryByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
						Return([]database.ReminderView{reminder}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: 43}), nil).
						Times(1),
				)
			},
		},
		{
			name: "settings screen is sent",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.sendSettings(ctx, chatID)
			},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					repository.EXPECT().
						UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{
							Timezone:          "UTC",
							IntrusiveInterval: 30 * time.Minute,
						}, nil).
						Times(1),
					repository.EXPECT().
						UserSettings(gomock.Any(), chatID).
						Return(database.UserSettings{
							Timezone:          "UTC",
							IntrusiveInterval: 30 * time.Minute,
						}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: 44}), nil).
						Times(1),
				)
			},
		},
		{
			name: "active reminder list service error is returned",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.sendReminderList(ctx, chatID)
			},
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().
					FindRemindersByChat(gomock.Any(), chatID, gomock.Any(), gomock.Any()).
					Return(nil, errLoadReminders).
					Times(1)
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: "load reminders: load reminders"},
		},
		{
			name: "settings service error is returned",
			action: func(ctx context.Context, bot *ReminderBot) error {
				return bot.sendSettings(ctx, chatID)
			},
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				repository.EXPECT().UserSettings(gomock.Any(), chatID).Return(database.UserSettings{}, errSettings).
					Times(1)
			},
			expected: struct {
				expectedErrText string
			}{expectedErrText: "load chat location settings: settings"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(caller, service, repository)

			botAPI, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:            botAPI,
				service:        service,
				repository:     repository,
				draftByChat:    map[int64]reminderDraft{},
				uiByChat:       map[int64]int{},
				locationByChat: make(map[int64]*time.Location),
			}

			err = test.action(t.Context(), bot)

			if test.expected.expectedErrText != "" {
				require.EqualError(t, err, test.expected.expectedErrText)

				return
			}

			require.NoError(t, err)
		})
	}
}

//nolint:lll
func TestReminderBotSavesDraft(t *testing.T) {
	t.Parallel()

	chatID := int64(500)
	messageID := 77
	futureDate := time.Date(2099, 6, 2, 0, 0, 0, 0, time.UTC)
	pastDate := time.Date(2000, 6, 2, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		draft    reminderDraft
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			expectedErr error
			draftExists bool
			screen      telegramui.Screen
			notice      string
		}
	}{
		{
			name:  "valid future draft is created and draft is deleted",
			draft: reminderDraft{title: "Pay rent", date: futureDate, hour: 9, minute: 30},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().
						CreateReminder(gomock.Any(), chatID, "Pay rent", "", gomock.Any()).
						Return(database.ReminderView{
							ID:       11,
							Title:    "Pay rent",
							RemindAt: futureDate.Add(9*time.Hour + 30*time.Minute),
						}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:  "valid future draft with description is created",
			draft: reminderDraft{title: "Call", description: "Mom", date: futureDate, hour: 12},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().
						CreateReminder(gomock.Any(), chatID, "Call", "Mom", gomock.Any()).
						Return(database.ReminderView{
							ID:          12,
							Title:       "Call",
							Description: "Mom",
							RemindAt:    futureDate.Add(12 * time.Hour),
						}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:  "valid evening draft is created",
			draft: reminderDraft{title: "Report", date: futureDate, hour: 18, minute: 5},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().
						CreateReminder(gomock.Any(), chatID, "Report", "", gomock.Any()).
						Return(database.ReminderView{
							ID:       13,
							Title:    "Report",
							RemindAt: futureDate.Add(18*time.Hour + 5*time.Minute),
						}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:  "valid midnight draft is created",
			draft: reminderDraft{title: "Night task", date: futureDate, hour: 23, minute: 55},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().
						CreateReminder(gomock.Any(), chatID, "Night task", "", gomock.Any()).
						Return(database.ReminderView{
							ID:       14,
							Title:    "Night task",
							RemindAt: futureDate.Add(23*time.Hour + 55*time.Minute),
						}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name: "valid custom weekdays draft is created with selected weekdays mask",
			draft: reminderDraft{
				title:        "Training",
				date:         futureDate,
				hour:         7,
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: 2 | 8,
			},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				gomock.InOrder(
					service.EXPECT().
						CreateWeekdaysReminder(gomock.Any(), chatID, "Training", "", gomock.Any(), 2|8).
						Return(database.ReminderView{
							ID:       15,
							Title:    "Training",
							RemindAt: futureDate.Add(7 * time.Hour),
						}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name:  "draft without title stays open with validation notice",
			draft: reminderDraft{date: futureDate, hour: 9},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(2)
			},
			expected: struct {
				expectedErr error
				draftExists bool
				screen      telegramui.Screen
				notice      string
			}{
				draftExists: true,
				screen:      telegramui.ScreenMain,
				notice:      "\u0427\u0442\u043e\u0431\u044b \u0441\u043e\u0445\u0440\u0430\u043d\u0438\u0442\u044c \u043d\u0430\u043f\u043e\u043c\u0438\u043d\u0430\u043d\u0438\u0435, \u0434\u043e\u0431\u0430\u0432\u044c\u0442\u0435 \u043d\u0430\u0437\u0432\u0430\u043d\u0438\u0435 \u0438 \u0434\u0430\u0442\u0443 \u0441 \u0432\u0440\u0435\u043c\u0435\u043d\u0435\u043c.",
			},
		},
		{
			name:  "past draft stays open with time validation notice",
			draft: reminderDraft{title: "Old task", date: pastDate, hour: 9},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(2)
			},
			expected: struct {
				expectedErr error
				draftExists bool
				screen      telegramui.Screen
				notice      string
			}{
				draftExists: true,
				screen:      telegramui.ScreenTime,
				notice:      "\u042d\u0442\u043e \u0432\u0440\u0435\u043c\u044f \u0443\u0436\u0435 \u043f\u0440\u043e\u0448\u043b\u043e. \u0412\u044b\u0431\u0435\u0440\u0438\u0442\u0435 \u0431\u0443\u0434\u0443\u0449\u0443\u044e \u0434\u0430\u0442\u0443 \u0438 \u0432\u0440\u0435\u043c\u044f.",
			},
		},
		{
			name: "custom weekdays draft without selected weekdays stays open",
			draft: reminderDraft{
				title:  "Training",
				date:   futureDate,
				hour:   7,
				repeat: telegramui.RepeatCustomWeekdays,
			},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(2)
			},
			expected: struct {
				expectedErr error
				draftExists bool
				screen      telegramui.Screen
				notice      string
			}{
				draftExists: true,
				screen:      telegramui.ScreenWeekdays,
				notice:      "Выберите хотя бы один день недели.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(caller, service, repository)

			botAPI, err := telego.NewBot(
				testTelegramToken,
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:         botAPI,
				service:     service,
				repository:  repository,
				draftByChat: map[int64]reminderDraft{chatID: test.draft},
				uiByChat:    map[int64]int{chatID: messageID},
			}

			err = bot.saveDraft(t.Context(), chatID, messageID)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())

				return
			}

			require.NoError(t, err)

			draft, exists := bot.draftByChat[chatID]
			require.Equal(t, test.expected.draftExists, exists)
			require.Equal(t, test.expected.screen, draft.screen)
			require.Equal(t, test.expected.notice, draft.notice)
		})
	}
}

func TestReminderBotCreatesDraftReminderVariants(t *testing.T) {
	t.Parallel()

	chatID := int64(510)
	remindAt := time.Date(2099, 6, 2, 9, 30, 0, 0, time.Local)
	customDate := time.Date(2099, 6, 4, 0, 0, 0, 0, time.Local)

	tests := []struct {
		name     string
		draft    reminderDraft
		before   func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository)
		expected struct {
			reminder        database.ReminderView
			expectedErrText string
		}
	}{
		{
			name:  "one time draft creates single reminder",
			draft: reminderDraft{title: "Pay rent", description: "Card"},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				service.EXPECT().
					CreateReminder(gomock.Any(), chatID, "Pay rent", "Card", remindAt).
					Return(database.ReminderView{ID: 21, Title: "Pay rent"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 21, Title: "Pay rent"}},
		},
		{
			name:  "workdays draft creates weekdays reminder with workdays mask",
			draft: reminderDraft{title: "Standup", repeat: telegramui.RepeatWeekdays},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				service.EXPECT().
					CreateWeekdaysReminder(gomock.Any(), chatID, "Standup", "", remindAt, database.WeekdaysMaskWorkdays).
					Return(database.ReminderView{ID: 22, Title: "Standup"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 22, Title: "Standup"}},
		},
		{
			name: "custom weekdays draft creates weekdays reminder with selected mask",
			draft: reminderDraft{
				title:        "Training",
				repeat:       telegramui.RepeatCustomWeekdays,
				weekdaysMask: database.WeekdayMaskTuesday | database.WeekdayMaskThursday,
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				service.EXPECT().
					CreateWeekdaysReminder(
						gomock.Any(),
						chatID,
						"Training",
						"",
						remindAt,
						database.WeekdayMaskTuesday|database.WeekdayMaskThursday,
					).
					Return(database.ReminderView{ID: 23, Title: "Training"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 23, Title: "Training"}},
		},
		{
			name:  "every two days draft creates interval reminder",
			draft: reminderDraft{title: "Medicine", repeat: telegramui.RepeatEveryTwoDays},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				service.EXPECT().
					CreateIntervalReminder(gomock.Any(), chatID, "Medicine", "", remindAt, 2).
					Return(database.ReminderView{ID: 24, Title: "Medicine"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 24, Title: "Medicine"}},
		},
		{
			name: "custom dates draft creates custom dates reminder",
			draft: reminderDraft{
				title:       "Trip",
				description: "Pack",
				repeat:      telegramui.RepeatCustomDates,
				customDates: []time.Time{customDate},
				hour:        8,
				minute:      15,
			},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				service.EXPECT().
					CreateCustomDatesReminder(
						gomock.Any(),
						chatID,
						"Trip",
						"Pack",
						[]time.Time{customDate},
						8,
						15,
						time.Local,
					).
					Return(database.ReminderView{ID: 25, Title: "Trip"}, nil).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{reminder: database.ReminderView{ID: 25, Title: "Trip"}},
		},
		{
			name:  "interval service error is returned",
			draft: reminderDraft{title: "Medicine", repeat: telegramui.RepeatEveryDay},
			before: func(service *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {
				service.EXPECT().
					CreateIntervalReminder(gomock.Any(), chatID, "Medicine", "", remindAt, 1).
					Return(database.ReminderView{}, errCreateDraftReminder).
					Times(1)
			},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "create interval reminder: create draft reminder"},
		},
		{
			name:   "unsupported repeat returns explicit error",
			draft:  reminderDraft{title: "Unknown", repeat: telegramui.Repeat("unsupported")},
			before: func(_ *telegrammocks.MockReminderService, repository *telegrammocks.MockReminderRepository) {},
			expected: struct {
				reminder        database.ReminderView
				expectedErrText string
			}{expectedErrText: "unsupported reminder repeat: unsupported"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			service := telegrammocks.NewMockReminderService(ctrl)
			repository := telegrammocks.NewMockReminderRepository(ctrl)
			test.before(service, repository)
			bot := &ReminderBot{service: service}

			reminder, err := bot.createDraftReminder(t.Context(), chatID, test.draft, remindAt)

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

func TestReminderBotConfigurationHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   func(t *testing.T)
		expected struct {
			expectedErr error
		}
	}{
		{
			name: "new reminder bot initializes state maps",
			action: func(t *testing.T) {
				t.Helper()

				ctrl := gomock.NewController(t)
				caller := telegoapimock.NewMockCaller(ctrl)
				botAPI, err := telego.NewBot(
					testTelegramToken,
					telego.WithAPICaller(caller),
					telego.WithDiscardLogger(),
				)
				require.NoError(t, err)

				bot := NewReminderBot(
					&Sender{bot: botAPI},
					telegrammocks.NewMockReminderService(ctrl),
					telegrammocks.NewMockReminderRepository(ctrl),
					config.Telegram{},
				)

				require.NotNil(t, bot.draftByChat)
				require.NotNil(t, bot.uiByChat)
			},
		},
		{
			name: "info returns reminder bot description",
			action: func(t *testing.T) {
				t.Helper()

				require.Equal(t, "run telegram reminder bot", (&ReminderBot{}).Info())
			},
		},
		{
			name: "long polling options are built from config",
			action: func(t *testing.T) {
				t.Helper()

				bot := &ReminderBot{cfg: config.Telegram{
					LongPollingBuffer:   10,
					LongPollingInterval: time.Second,
					LongPollingRetry:    2 * time.Second,
				}}

				require.Len(t, bot.longPollingOptions(), 3)
			},
		},
		{
			name: "bot uses server local timezone when telegram has no user timezone",
			action: func(t *testing.T) {
				t.Helper()

				location := (&ReminderBot{}).location(1)

				require.Equal(t, time.Local, location)
			},
		},
		{
			name: "next reminder time rolls minutes into next hour",
			action: func(t *testing.T) {
				t.Helper()

				next := nextReminderTime(time.Date(2026, 6, 1, 10, 58, 0, 0, time.UTC))

				require.Equal(t, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), next)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			test.action(t)
		})
	}
}
