//nolint:lll,testpackage,goconst // Tests cover internal Telegram payloads with readable inline fixtures.
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
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
	telegoapimock "github.com/mymmrac/telego/telegoapi/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errTelegramAPI = errors.New("telegram api")
var testTelegramMessageID = 40

var testTelegramToken = strings.Join([]string{"1234567890", strings.Repeat("a", 35)}, ":")

func telegramResponse(t *testing.T, value any) *telegoapi.Response {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return &telegoapi.Response{Ok: true, Result: data}
}

func TestSenderSendReminder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      config.Telegram
		reminder Reminder
		before   func(caller *telegoapimock.MockCaller)
		expected struct {
			messageID       int
			expectedErrText string
		}
	}{
		{
			name: "title only reminder is sent",
			reminder: Reminder{
				ID:             1,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 1}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 1},
		},
		{
			name: "description is appended to reminder text",
			reminder: Reminder{
				ID:             2,
				TelegramChatID: 100,
				Title:          "Pay rent",
				Description:    "Before lunch",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 2}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 2},
		},
		{
			name: "parse mode reminder is sent",
			cfg:  config.Telegram{ParseMode: telego.ModeHTML},
			reminder: Reminder{
				ID:             3,
				TelegramChatID: 100,
				Title:          "<b>Pay rent</b>",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 3}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 3},
		},
		{
			name: "silent protected reminder is sent",
			cfg:  config.Telegram{DisableNotification: true, ProtectContent: true},
			reminder: Reminder{
				ID:             4,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 4}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 4},
		},
		{
			name: "previous reminder message is deleted before new reminder is sent",
			reminder: Reminder{
				ID:                7,
				TelegramChatID:    100,
				Title:             "Pay rent",
				TelegramMessageID: &testTelegramMessageID,
			},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 7}), nil).
					Times(3)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 7},
		},
		{
			name: "api caller error is returned",
			reminder: Reminder{
				ID:             5,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				gomock.InOrder(
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(telegramResponse(t, telego.Message{MessageID: 5}), nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errTelegramAPI).
						Times(1),
				)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{expectedErrText: "send telegram reminder 5: telego: sendMessage: internal execution: request call: telegram api"},
		},
		{
			name: "api response error is returned",
			reminder: Reminder{
				ID:             6,
				TelegramChatID: 100,
				Title:          "Pay rent",
			},
			before: func(caller *telegoapimock.MockCaller) {
				gomock.InOrder(
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(telegramResponse(t, telego.Message{MessageID: 6}), nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&telegoapi.Response{
							Ok:    false,
							Error: &telegoapi.Error{ErrorCode: 400, Description: "bad request"},
						}, nil).
						Times(1),
				)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{expectedErrText: "send telegram reminder 6: telego: sendMessage: api: 400 \"bad request\""},
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

			messageID, err := NewWithBot(bot, test.cfg).SendReminder(t.Context(), test.reminder)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected send reminder error")
				require.Equal(t, test.expected.messageID, messageID)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.messageID, messageID)
		})
	}
}

func TestSenderSendHoliday(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      config.Telegram
		holiday  Holiday
		before   func(caller *telegoapimock.MockCaller)
		expected struct {
			messageID       int
			expectedErrText string
		}
	}{
		{
			name:    "holiday is sent",
			holiday: Holiday{ID: 1, TelegramChatID: 100, HolidayName: "Birthday", RecipientName: "Oleg"},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 1}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 1},
		},
		{
			name:    "parse mode holiday is sent",
			cfg:     config.Telegram{ParseMode: telego.ModeHTML},
			holiday: Holiday{ID: 2, TelegramChatID: 100, HolidayName: "<b>Birthday</b>", RecipientName: "Oleg"},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 2}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 2},
		},
		{
			name:    "silent holiday is sent",
			cfg:     config.Telegram{DisableNotification: true},
			holiday: Holiday{ID: 3, TelegramChatID: 100, HolidayName: "Birthday", RecipientName: "Oleg"},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 3}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 3},
		},
		{
			name:    "protected holiday is sent",
			cfg:     config.Telegram{ProtectContent: true},
			holiday: Holiday{ID: 4, TelegramChatID: 100, HolidayName: "Birthday", RecipientName: "Oleg"},
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 4}), nil).
					Times(2)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 4},
		},
		{
			name:    "holiday send api caller error is returned",
			holiday: Holiday{ID: 5, TelegramChatID: 100, HolidayName: "Birthday", RecipientName: "Oleg"},
			before: func(caller *telegoapimock.MockCaller) {
				gomock.InOrder(
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(telegramResponse(t, telego.Message{MessageID: 5}), nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errTelegramAPI).
						Times(1),
				)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{expectedErrText: "send telegram holiday 5: telego: sendMessage: internal execution: request call: telegram api"},
		},
		{
			name:    "holiday api response error is returned",
			holiday: Holiday{ID: 6, TelegramChatID: 100, HolidayName: "Birthday", RecipientName: "Oleg"},
			before: func(caller *telegoapimock.MockCaller) {
				gomock.InOrder(
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(telegramResponse(t, telego.Message{MessageID: 6}), nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&telegoapi.Response{
							Ok:    false,
							Error: &telegoapi.Error{ErrorCode: 400, Description: "bad request"},
						}, nil).
						Times(1),
				)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{expectedErrText: "send telegram holiday 6: telego: sendMessage: api: 400 \"bad request\""},
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

			messageID, err := NewWithBot(bot, test.cfg).SendHoliday(t.Context(), test.holiday)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected send holiday error")
				require.Equal(t, test.expected.messageID, messageID)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.messageID, messageID)
		})
	}
}

func TestNewSender(t *testing.T) {
	tests := []struct {
		name     string
		tokenEnv string
		token    string
		cfg      config.Telegram
		expected struct {
			expectedErrText string
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
			cfg:      config.Telegram{Warnings: false},
		},
		{
			name:     "custom api base url is accepted",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_C",
			token:    testTelegramToken,
			cfg:      config.Telegram{APIBaseURL: "https://api.telegram.org"},
		},
		{
			name:     "request timeout is accepted",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_D",
			token:    testTelegramToken,
			cfg:      config.Telegram{RequestTimeout: time.Second},
		},
		{
			name:     "missing token returns explicit error",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_E",
			expected: struct{ expectedErrText string }{
				expectedErrText: "telegram token is empty: env EASY_NOTE_TEST_TOKEN_E",
			},
		},
		{
			name:     "empty token returns explicit error",
			tokenEnv: "EASY_NOTE_TEST_TOKEN_F",
			token:    "",
			expected: struct{ expectedErrText string }{
				expectedErrText: "telegram token is empty: env EASY_NOTE_TEST_TOKEN_F",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.tokenEnv, test.token)

			cfg := test.cfg

			cfg.TokenEnv = test.tokenEnv
			if cfg.APIBaseURL == "" {
				cfg.APIBaseURL = "https://api.telegram.org"
			}

			if cfg.RequestTimeout == 0 {
				cfg.RequestTimeout = time.Second
			}

			if cfg.LongPollingRetry == 0 {
				cfg.LongPollingRetry = time.Second
			}

			sender, err := NewSender(context.Background(), cfg)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected new sender error")
				require.Nil(t, sender)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, sender)
		})
	}
}

func TestSchedulerSender(t *testing.T) {
	t.Parallel()

	reminder := database.ScheduledReminder{
		ID:             1,
		TelegramChatID: 100,
		Title:          "Pay rent",
	}
	reminderWithMessage := reminder
	reminderWithMessage.ID = 2
	reminderWithMessage.TelegramMessageID = &testTelegramMessageID
	holiday := database.ScheduledHoliday{
		ID:             3,
		TelegramChatID: 100,
		HolidayName:    "Birthday",
		RecipientName:  "Oleg",
	}

	tests := []struct {
		name     string
		before   func(caller *telegoapimock.MockCaller)
		action   func(sender *SchedulerSender) (int, error)
		expected struct {
			messageID       int
			expectedErrText string
		}
	}{
		{
			name: "scheduler reminder is sent",
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 1}), nil).
					Times(2)
			},
			action: func(sender *SchedulerSender) (int, error) {
				return sender.SendReminder(t.Context(), reminder)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 1},
		},
		{
			name: "scheduler reminder with previous message deletes and sends",
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 2}), nil).
					Times(3)
			},
			action: func(sender *SchedulerSender) (int, error) {
				return sender.SendReminder(t.Context(), reminderWithMessage)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 2},
		},
		{
			name: "scheduler holiday is sent",
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, telego.Message{MessageID: 3}), nil).
					Times(2)
			},
			action: func(sender *SchedulerSender) (int, error) {
				return sender.SendHoliday(t.Context(), holiday)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{messageID: 3},
		},
		{
			name: "scheduler reminder message is deleted",
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(telegramResponse(t, true), nil).
					Times(1)
			},
			action: func(sender *SchedulerSender) (int, error) {
				return 0, sender.DeleteReminderMessage(t.Context(), reminderWithMessage)
			},
		},
		{
			name: "scheduler reminder api error is returned",
			before: func(caller *telegoapimock.MockCaller) {
				gomock.InOrder(
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(telegramResponse(t, telego.Message{MessageID: 1}), nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errTelegramAPI).
						Times(1),
				)
			},
			action: func(sender *SchedulerSender) (int, error) {
				return sender.SendReminder(t.Context(), reminder)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{expectedErrText: "send telegram reminder 1: telego: sendMessage: internal execution: request call: telegram api"},
		},
		{
			name: "scheduler holiday api error is returned",
			before: func(caller *telegoapimock.MockCaller) {
				gomock.InOrder(
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(telegramResponse(t, telego.Message{MessageID: 3}), nil).
						Times(1),
					caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errTelegramAPI).
						Times(1),
				)
			},
			action: func(sender *SchedulerSender) (int, error) {
				return sender.SendHoliday(t.Context(), holiday)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{expectedErrText: "send telegram holiday 3: telego: sendMessage: internal execution: request call: telegram api"},
		},
		{
			name: "scheduler delete reminder message api error is returned",
			before: func(caller *telegoapimock.MockCaller) {
				caller.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errTelegramAPI).
					Times(1)
			},
			action: func(sender *SchedulerSender) (int, error) {
				return 0, sender.DeleteReminderMessage(t.Context(), reminderWithMessage)
			},
			expected: struct {
				messageID       int
				expectedErrText string
			}{expectedErrText: "delete telegram reminder 2 message: telego: deleteMessage: internal execution: request call: telegram api"},
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

			messageID, err := test.action(NewSchedulerSender(NewWithBot(bot, config.Telegram{})))

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected scheduler sender error")
				require.Equal(t, test.expected.messageID, messageID)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.messageID, messageID)
		})
	}
}
