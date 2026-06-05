//nolint:testpackage // These tests cover internal Telegram UI state transitions without adding production hooks.
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	telegrammocks "github.com/EgorKo25/EasyNote/src/telegram/mocks"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
	telegoapimock "github.com/mymmrac/telego/telegoapi/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errTelegramAPI = errors.New("telegram api")

var (
	errExpectedSendCaller = errors.New(
		"send telegram reminder 5: telego: sendMessage: internal execution: request call: telegram api",
	)
	errExpectedSendResponse   = errors.New("send telegram reminder 6: telego: sendMessage: api: 400 \"bad request\"")
	errExpectedEmptyTokenEnvE = errors.New("telegram token is empty: env EASY_NOTE_TEST_TOKEN_E")
	errExpectedEmptyTokenEnvF = errors.New("telegram token is empty: env EASY_NOTE_TEST_TOKEN_F")
	errLoadReminders          = errors.New("load reminders")
	errExpectedLoadReminders  = errors.New("load reminders: load reminders")
	errCreateDraftReminder    = errors.New("create draft reminder")
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
			waitingFor  draftField
			screen      draftScreen
			notice      string
		}
	}{
		{
			name:  "title is saved from user message",
			draft: reminderDraft{waitingFor: fieldTitle},
			input: "Pay rent",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  draftField
				screen      draftScreen
				notice      string
			}{applied: true, title: "Pay rent", screen: draftScreenMain},
		},
		{
			name:  "description is saved from user message",
			draft: reminderDraft{waitingFor: fieldDescription},
			input: "Before lunch",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  draftField
				screen      draftScreen
				notice      string
			}{applied: true, description: "Before lunch", screen: draftScreenMain},
		},
		{
			name:  "short title is saved from user message",
			draft: reminderDraft{waitingFor: fieldTitle},
			input: "Call",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  draftField
				screen      draftScreen
				notice      string
			}{applied: true, title: "Call", screen: draftScreenMain},
		},
		{
			name:  "long description is saved from user message",
			draft: reminderDraft{waitingFor: fieldDescription},
			input: "Prepare documents before lunch",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  draftField
				screen      draftScreen
				notice      string
			}{applied: true, description: "Prepare documents before lunch", screen: draftScreenMain},
		},
		{
			name:  "message is ignored when draft does not wait for text",
			draft: reminderDraft{screen: draftScreenMain},
			input: "ignored",
			expected: struct {
				applied     bool
				title       string
				description string
				hour        int
				minute      int
				waitingFor  draftField
				screen      draftScreen
				notice      string
			}{applied: false, screen: draftScreenMain},
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
				waitingFor  draftField
				screen      draftScreen
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

			applied := bot.applyDraftText(chatID, test.input)
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
			screen  draftScreen
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
				screen  draftScreen
			}{applied: true, date: time.Date(2099, 6, 4, 0, 0, 0, 0, location), screen: draftScreenTime},
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
				screen  draftScreen
			}{applied: true, screen: draftScreenCalendar},
		},
		{
			name:  "time preset callback changes selected time",
			draft: reminderDraft{date: time.Date(2026, 6, 2, 0, 0, 0, 0, location), screen: draftScreenTime},
			data:  "time:set:18:00",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  draftScreen
			}{applied: true, date: time.Date(2026, 6, 2, 0, 0, 0, 0, location), hour: 18, screen: draftScreenTime},
		},
		{
			name: "time shift callback moves selected time",
			draft: reminderDraft{
				date:   time.Date(2026, 6, 2, 0, 0, 0, 0, location),
				hour:   10,
				minute: 5,
				screen: draftScreenTime,
			},
			data: "time:shift:-5",
			expected: struct {
				applied bool
				date    time.Time
				hour    int
				minute  int
				screen  draftScreen
			}{applied: true, date: time.Date(2026, 6, 2, 0, 0, 0, 0, location), hour: 10, screen: draftScreenTime},
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
				screen  draftScreen
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
				screen  draftScreen
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
			repeat       reminderRepeat
			weekdaysMask int
			screen       draftScreen
		}
	}{
		{
			name:  "monday callback selects monday for custom weekdays",
			draft: reminderDraft{},
			data:  "weekday:1",
			expected: struct {
				applied      bool
				repeat       reminderRepeat
				weekdaysMask int
				screen       draftScreen
			}{applied: true, repeat: reminderRepeatCustomWeekdays, weekdaysMask: 1, screen: draftScreenWeekdays},
		},
		{
			name: "thursday callback adds thursday to existing monday",
			draft: reminderDraft{
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: 1,
				screen:       draftScreenWeekdays,
			},
			data: "weekday:8",
			expected: struct {
				applied      bool
				repeat       reminderRepeat
				weekdaysMask int
				screen       draftScreen
			}{
				applied:      true,
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: 1 | 8,
				screen:       draftScreenWeekdays,
			},
		},
		{
			name: "selected weekday callback removes weekday from mask",
			draft: reminderDraft{
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: 1 | 8,
				screen:       draftScreenWeekdays,
			},
			data: "weekday:8",
			expected: struct {
				applied      bool
				repeat       reminderRepeat
				weekdaysMask int
				screen       draftScreen
			}{
				applied:      true,
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: 1,
				screen:       draftScreenWeekdays,
			},
		},
		{
			name: "sunday callback preserves existing weekend mask",
			draft: reminderDraft{
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: 32,
				screen:       draftScreenWeekdays,
			},
			data: "weekday:64",
			expected: struct {
				applied      bool
				repeat       reminderRepeat
				weekdaysMask int
				screen       draftScreen
			}{
				applied:      true,
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: 32 | 64,
				screen:       draftScreenWeekdays,
			},
		},
		{
			name:  "invalid weekday callback is rejected",
			draft: reminderDraft{},
			data:  "weekday:not-a-number",
			expected: struct {
				applied      bool
				repeat       reminderRepeat
				weekdaysMask int
				screen       draftScreen
			}{applied: false},
		},
		{
			name:  "unknown callback is rejected",
			draft: reminderDraft{},
			data:  "weekday-unknown:1",
			expected: struct {
				applied      bool
				repeat       reminderRepeat
				weekdaysMask int
				screen       draftScreen
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
			name:   "main menu text is concise",
			action: mainMenuText,
			expected: struct {
				expectedText string
			}{
				expectedText: "Главное меню\n\n" +
					"Выберите действие на клавиатуре ниже.",
			},
		},
		{
			name: "draft text shows selected date time and empty repeat",
			action: func() string {
				return draftText(reminderDraft{
					title:       "Pay rent",
					description: "Before lunch",
					date:        time.Date(2026, 6, 2, 0, 0, 0, 0, location),
					hour:        9,
					minute:      30,
				}, location)
			},
			expected: struct {
				expectedText string
			}{
				expectedText: "Создание напоминания\n\n" +
					"Дата и время: 02.06.2026 09:30\n" +
					"Название: Pay rent\n" +
					"Описание: Before lunch\n" +
					"Повторяемость: не задана\n\n" +
					"Выберите дату и время кнопкой ниже.",
			},
		},
		{
			name: "draft text shows selected custom date count",
			action: func() string {
				return draftText(reminderDraft{
					title:       "Pay rent",
					description: "Before lunch",
					repeat:      reminderRepeatCustomDates,
					customDates: []time.Time{
						time.Date(2026, 6, 2, 0, 0, 0, 0, location),
						time.Date(2026, 6, 4, 0, 0, 0, 0, location),
					},
				}, location)
			},
			expected: struct {
				expectedText string
			}{
				expectedText: "Создание напоминания\n\n" +
					"Дата и время: не задано\n" +
					"Название: Pay rent\n" +
					"Описание: Before lunch\n" +
					"Повторяемость: выбрано дат: 2\n\n" +
					"Выберите дату и время кнопкой ниже.",
			},
		},
		{
			name: "reminder view text includes description",
			action: func() string {
				return reminderViewText(reminder, location)
			},
			expected: struct {
				expectedText string
			}{expectedText: "02.06.2026 09:30\nPay rent\nBefore lunch"},
		},
		{
			name: "empty draft text shows missing fields and empty repeat",
			action: func() string {
				return draftText(reminderDraft{}, location)
			},
			expected: struct {
				expectedText string
			}{
				expectedText: "Создание напоминания\n\n" +
					"Дата и время: не задано\n" +
					"Название: не задано\n" +
					"Описание: не задано\n" +
					"Повторяемость: не задана\n\n" +
					"Выберите дату и время кнопкой ниже.",
			},
		},
		{
			name: "empty reminder list explains next action",
			action: func() string {
				return reminderListText(nil)
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
		action   func() *telego.InlineKeyboardMarkup
		expected struct {
			rowCount        int
			firstButtonText string
		}
	}{
		{
			name: "main draft keyboard has three command rows",
			action: func() *telego.InlineKeyboardMarkup {
				return draftKeyboard(reminderDraft{}, now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Дата"},
		},
		{
			name: "filled one time draft keyboard offers changing date",
			action: func() *telego.InlineKeyboardMarkup {
				return draftKeyboard(reminderDraft{
					date: time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local),
				}, now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Изменить дату"},
		},
		{
			name: "empty recurring draft keyboard explains start date time",
			action: func() *telego.InlineKeyboardMarkup {
				return draftKeyboard(reminderDraft{repeat: reminderRepeatEveryDay}, now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Дата и время начала"},
		},
		{
			name: "filled recurring draft keyboard offers changing start date time",
			action: func() *telego.InlineKeyboardMarkup {
				return draftKeyboard(reminderDraft{
					date:   time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local),
					repeat: reminderRepeatEveryDay,
				}, now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4, firstButtonText: "Изменить дату и время начала"},
		},
		{
			name: "date choice keyboard is compact",
			action: func() *telego.InlineKeyboardMarkup {
				return draftKeyboard(reminderDraft{screen: draftScreenDate}, now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 3},
		},
		{
			name: "calendar keyboard opens explicit calendar grid",
			action: func() *telego.InlineKeyboardMarkup {
				return draftKeyboard(reminderDraft{
					screen:        draftScreenCalendar,
					calendarMonth: time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local),
				}, now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 8},
		},
		{
			name: "time keyboard has future presets, steppers and done",
			action: func() *telego.InlineKeyboardMarkup {
				return draftKeyboard(reminderDraft{
					screen: draftScreenTime,
					date:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local),
					hour:   9, minute: 30,
				}, now)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 4},
		},
		{
			name: "empty reminder list keyboard still has main menu",
			action: func() *telego.InlineKeyboardMarkup {
				return reminderListKeyboard(nil)
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 1},
		},
		{
			name: "reminder list keyboard opens reminders by title buttons",
			action: func() *telego.InlineKeyboardMarkup {
				return reminderListKeyboard([]database.ReminderView{
					{ID: 1, Title: "Pay rent"},
					{ID: 2, Title: "Call mom"},
				})
			},
			expected: struct {
				rowCount        int
				firstButtonText string
			}{rowCount: 3, firstButtonText: "Pay rent"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			keyboard := test.action()

			require.Len(t, keyboard.InlineKeyboard, test.expected.rowCount)

			if test.expected.firstButtonText != "" {
				require.Equal(t, test.expected.firstButtonText, keyboard.InlineKeyboard[0][0].Text)
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
				"1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			sender := &Sender{bot: bot, cfg: test.cfg}
			err = sender.SendReminder(t.Context(), test.reminder)

			if test.expected.expectedErr != nil {
				require.EqualError(t, err, test.expected.expectedErr.Error())

				return
			}

			require.NoError(t, err)
		})
	}
}

//nolint:gosec
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
			token:    "1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
		},
		{
			name:     "warnings can be disabled",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_B",
			token:    "1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
		},
		{
			name:     "custom api base url is accepted",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_C",
			token:    "1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
		},
		{
			name:     "request timeout is accepted",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_D",
			token:    "1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
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

func TestReminderBotHandlesMessages(t *testing.T) {
	t.Parallel()

	chatID := int64(400)
	messageID := 55

	tests := []struct {
		name     string
		message  telego.Message
		draft    reminderDraft
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService)
		expected struct {
			expectedErr error
			draftTitle  string
		}
	}{
		{
			name:    "start command opens main menu",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: "/start"},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 1}), nil).
					Times(1)
			},
		},
		{
			name:    "create reminder command opens draft",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: buttonCreateReminder},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 2}), nil).
					Times(2)
			},
		},
		{
			name:    "reminder list command shows reminders",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: buttonReminderList},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
				gomock.InOrder(
					service.EXPECT().Reminders(gomock.Any(), chatID).Return([]database.ReminderView{{
						ID:       1,
						Title:    "Pay rent",
						RemindAt: time.Date(2026, 6, 2, 9, 30, 0, 0, time.Local),
					}}, nil).Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: 3}), nil).
						Times(2),
				)
			},
		},
		{
			name:    "draft title message updates current draft",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: "Pay rent"},
			draft:   reminderDraft{waitingFor: fieldTitle},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
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
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: 5}), nil).
					Times(2)
			},
		},
		{
			name:    "reminder list service error is returned",
			message: telego.Message{MessageID: messageID, Chat: telego.Chat{ID: chatID}, Text: buttonReminderList},
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
				service.EXPECT().Reminders(gomock.Any(), chatID).Return(nil, errLoadReminders).
					Times(1)
			},
			expected: struct {
				expectedErr error
				draftTitle  string
			}{expectedErr: errExpectedLoadReminders},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			caller := telegoapimock.NewMockCaller(ctrl)
			service := telegrammocks.NewMockReminderService(ctrl)
			test.before(caller, service)

			botAPI, err := telego.NewBot(
				"1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
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
			screen  draftScreen
			waiting draftField
		}
	}{
		{
			name:  "title callback waits for title",
			data:  callbackDraftTitle,
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  draftScreen
				waiting draftField
			}{handled: true, screen: draftScreenMain, waiting: fieldTitle},
		},
		{
			name:  "description callback waits for description",
			data:  callbackDraftDescription,
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  draftScreen
				waiting draftField
			}{handled: true, screen: draftScreenMain, waiting: fieldDescription},
		},
		{
			name:  "calendar callback opens calendar",
			data:  callbackDraftCalendar,
			draft: reminderDraft{screen: draftScreenDate},
			expected: struct {
				handled bool
				screen  draftScreen
				waiting draftField
			}{handled: true, screen: draftScreenCalendar},
		},
		{
			name:  "date callback opens compact date choice",
			data:  callbackDraftDate,
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  draftScreen
				waiting draftField
			}{handled: true, screen: draftScreenDate},
		},
		{
			name:  "unknown callback is not handled",
			data:  "unknown",
			draft: reminderDraft{},
			expected: struct {
				handled bool
				screen  draftScreen
				waiting draftField
			}{handled: false},
		},
		{
			name:  "noop callback is not a draft command",
			data:  callbackNoop,
			draft: reminderDraft{screen: draftScreenMain},
			expected: struct {
				handled bool
				screen  draftScreen
				waiting draftField
			}{handled: false, screen: draftScreenMain},
		},
		{
			name:  "back callback returns to main draft",
			data:  callbackDraftBack,
			draft: reminderDraft{screen: draftScreenCalendar},
			expected: struct {
				handled bool
				screen  draftScreen
				waiting draftField
			}{handled: true, screen: draftScreenMain},
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
				"1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
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
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService)
		expected struct {
			expectedErrText string
			draftExists     bool
		}
	}{
		{
			name: "create reminder callback starts draft in current message",
			data: callbackCreateReminder,
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
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
			data: callbackReminderList,
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
				gomock.InOrder(
					service.EXPECT().Reminders(gomock.Any(), chatID).
						Return([]database.ReminderView{reminder}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name: "view reminder callback edits reminder details",
			data: callbackViewPrefix + "10",
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
				gomock.InOrder(
					service.EXPECT().Reminders(gomock.Any(), chatID).
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
			data: callbackViewPrefix + "99",
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
				gomock.InOrder(
					service.EXPECT().Reminders(gomock.Any(), chatID).
						Return([]database.ReminderView{reminder}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name: "delete reminder callback deletes reminder and edits list",
			data: callbackDeletePrefix + "10",
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
				gomock.InOrder(
					service.EXPECT().DeleteReminder(gomock.Any(), chatID, int64(10)).
						Return(nil).
						Times(1),
					service.EXPECT().Reminders(gomock.Any(), chatID).
						Return([]database.ReminderView{}, nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
						Times(1),
				)
			},
		},
		{
			name: "invalid view callback returns parse error",
			data: callbackViewPrefix + "bad",
			before: func(_ *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
			},
			expected: struct {
				expectedErrText string
				draftExists     bool
			}{expectedErrText: `parse reminder id: strconv.ParseInt: parsing "bad": invalid syntax`},
		},
		{
			name: "view reminder service error is returned",
			data: callbackViewPrefix + "10",
			before: func(_ *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
				service.EXPECT().Reminders(gomock.Any(), chatID).
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
			test.before(caller, service)

			botAPI, err := telego.NewBot(
				"1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:         botAPI,
				service:     service,
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
		before   func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService)
		expected struct {
			expectedErr error
			draftExists bool
			screen      draftScreen
			notice      string
		}
	}{
		{
			name:  "valid future draft is created and draft is deleted",
			draft: reminderDraft{title: "Pay rent", date: futureDate, hour: 9, minute: 30},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
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
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
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
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
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
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
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
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: 2 | 8,
			},
			before: func(caller *telegoapimock.MockCaller, service *telegrammocks.MockReminderService) {
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
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(1)
			},
			expected: struct {
				expectedErr error
				draftExists bool
				screen      draftScreen
				notice      string
			}{
				draftExists: true,
				screen:      draftScreenMain,
				notice:      "\u0427\u0442\u043e\u0431\u044b \u0441\u043e\u0445\u0440\u0430\u043d\u0438\u0442\u044c \u043d\u0430\u043f\u043e\u043c\u0438\u043d\u0430\u043d\u0438\u0435, \u0434\u043e\u0431\u0430\u0432\u044c\u0442\u0435 \u043d\u0430\u0437\u0432\u0430\u043d\u0438\u0435 \u0438 \u0434\u0430\u0442\u0443 \u0441 \u0432\u0440\u0435\u043c\u0435\u043d\u0435\u043c.",
			},
		},
		{
			name:  "past draft stays open with time validation notice",
			draft: reminderDraft{title: "Old task", date: pastDate, hour: 9},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(1)
			},
			expected: struct {
				expectedErr error
				draftExists bool
				screen      draftScreen
				notice      string
			}{
				draftExists: true,
				screen:      draftScreenTime,
				notice:      "\u042d\u0442\u043e \u0432\u0440\u0435\u043c\u044f \u0443\u0436\u0435 \u043f\u0440\u043e\u0448\u043b\u043e. \u0412\u044b\u0431\u0435\u0440\u0438\u0442\u0435 \u0431\u0443\u0434\u0443\u0449\u0443\u044e \u0434\u0430\u0442\u0443 \u0438 \u0432\u0440\u0435\u043c\u044f.",
			},
		},
		{
			name: "custom weekdays draft without selected weekdays stays open",
			draft: reminderDraft{
				title:  "Training",
				date:   futureDate,
				hour:   7,
				repeat: reminderRepeatCustomWeekdays,
			},
			before: func(caller *telegoapimock.MockCaller, _ *telegrammocks.MockReminderService) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testTelegramResponse(t, telego.Message{MessageID: messageID}), nil).
					Times(1)
			},
			expected: struct {
				expectedErr error
				draftExists bool
				screen      draftScreen
				notice      string
			}{
				draftExists: true,
				screen:      draftScreenWeekdays,
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
			test.before(caller, service)

			botAPI, err := telego.NewBot(
				"1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
				telego.WithAPICaller(caller),
				telego.WithDiscardLogger(),
			)
			require.NoError(t, err)

			bot := &ReminderBot{
				bot:         botAPI,
				service:     service,
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
		before   func(service *telegrammocks.MockReminderService)
		expected struct {
			reminder        database.ReminderView
			expectedErrText string
		}
	}{
		{
			name:  "one time draft creates single reminder",
			draft: reminderDraft{title: "Pay rent", description: "Card"},
			before: func(service *telegrammocks.MockReminderService) {
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
			draft: reminderDraft{title: "Standup", repeat: reminderRepeatWeekdays},
			before: func(service *telegrammocks.MockReminderService) {
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
				repeat:       reminderRepeatCustomWeekdays,
				weekdaysMask: database.WeekdayMaskTuesday | database.WeekdayMaskThursday,
			},
			before: func(service *telegrammocks.MockReminderService) {
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
			draft: reminderDraft{title: "Medicine", repeat: reminderRepeatEveryTwoDays},
			before: func(service *telegrammocks.MockReminderService) {
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
				repeat:      reminderRepeatCustomDates,
				customDates: []time.Time{customDate},
				hour:        8,
				minute:      15,
			},
			before: func(service *telegrammocks.MockReminderService) {
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
			draft: reminderDraft{title: "Medicine", repeat: reminderRepeatEveryDay},
			before: func(service *telegrammocks.MockReminderService) {
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
			draft:  reminderDraft{title: "Unknown", repeat: reminderRepeat("unsupported")},
			before: func(_ *telegrammocks.MockReminderService) {},
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
			test.before(service)
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
					"1234567890:aaaabbbbaaaabbbbaaaabbbbaaaabbbbccc",
					telego.WithAPICaller(caller),
					telego.WithDiscardLogger(),
				)
				require.NoError(t, err)

				bot := NewReminderBot(
					&Sender{bot: botAPI},
					telegrammocks.NewMockReminderService(ctrl),
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

				location := (&ReminderBot{}).location()

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
