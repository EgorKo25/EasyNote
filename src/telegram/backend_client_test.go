//nolint:lll,testpackage,goconst // Tests cover backend client fixtures with inline JSON and internal response models.
package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	telegrammodel "github.com/EgorKo25/EasyNote/src/telegram/model"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/stretchr/testify/require"
)

func TestBackendClientSendsTelegramChatRequests(t *testing.T) {
	t.Parallel()

	chatID := int64(123456)
	remindAt := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	holidayDate := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		baseURLSuffix  string
		responseStatus int
		responseBody   string
		action         func(ctx context.Context, client *BackendClient) (string, error)
		expected       struct {
			result          string
			requestPath     string
			telegramHeader  string
			expectedErrText string
		}
	}{
		{
			name:           "dashboard returns reminder stats",
			responseStatus: http.StatusOK,
			responseBody:   `{"stats":{"completed":3,"pending":2,"skipped":1}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				stats, err := client.Dashboard(ctx, chatID)

				return fmt.Sprintf("%d/%d/%d", stats.Completed, stats.Pending, stats.Skipped), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "3/2/1",
				requestPath:    "/api/v1/dashboard",
				telegramHeader: "123456",
			},
		},
		{
			name:           "dashboard trims trailing base url slash",
			baseURLSuffix:  "/",
			responseStatus: http.StatusOK,
			responseBody:   `{"stats":{"completed":4,"pending":5,"skipped":6}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				stats, err := client.Dashboard(ctx, chatID)

				return fmt.Sprintf("%d/%d/%d", stats.Completed, stats.Pending, stats.Skipped), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "4/5/6",
				requestPath:    "/api/v1/dashboard",
				telegramHeader: "123456",
			},
		},
		{
			name:           "settings returns timezone and interval",
			responseStatus: http.StatusOK,
			responseBody:   `{"settings":{"telegram_chat_id":123456,"language":"ru","timezone":"Europe/Moscow","intrusive_enabled":true,"intrusive_interval_minutes":30,"updated_at":"2026-07-20T10:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				settings, err := client.Settings(ctx, chatID)

				return settings.Timezone + "|" + settings.IntrusiveInterval.String(), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "Europe/Moscow|30m0s",
				requestPath:    "/api/v1/settings",
				telegramHeader: "123456",
			},
		},
		{
			name:           "settings returns disabled intrusive settings",
			responseStatus: http.StatusOK,
			responseBody:   `{"settings":{"telegram_chat_id":123456,"language":"ru","timezone":"Asia/Novosibirsk","intrusive_enabled":false,"intrusive_interval_minutes":45,"updated_at":"2026-07-20T10:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				settings, err := client.Settings(ctx, chatID)

				return fmt.Sprintf(
					"%s|%t|%s",
					settings.Timezone,
					settings.IntrusiveEnabled,
					settings.IntrusiveInterval,
				), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "Asia/Novosibirsk|false|45m0s",
				requestPath:    "/api/v1/settings",
				telegramHeader: "123456",
			},
		},
		{
			name:           "active reminders are returned",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminders":[{"id":7,"rule_id":null,"title":"Pay rent","description":"Before lunch","remind_at":"2026-07-20T10:00:00Z","status":"new","created_at":"2026-07-20T09:00:00Z"}]}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminders, err := client.FindRemindersByChat(ctx, chatID, 2, 4)

				return fmt.Sprintf("%d|%s|%s", len(reminders), reminders[0].Title, reminders[0].Status), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "1|Pay rent|new",
				requestPath:    "/api/v1/reminders?limit=2&offset=4",
				telegramHeader: "123456",
			},
		},
		{
			name:           "history reminders are returned",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminders":[{"id":8,"rule_id":null,"title":"Archive","description":"","remind_at":"2026-07-19T10:00:00Z","status":"canceled","created_at":"2026-07-19T09:00:00Z"}]}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminders, err := client.FindReminderHistoryByChat(ctx, chatID, 3, 6)

				return fmt.Sprintf("%d|%s|%s", len(reminders), reminders[0].Title, reminders[0].Status), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "1|Archive|canceled",
				requestPath:    "/api/v1/reminders/history?limit=3&offset=6",
				telegramHeader: "123456",
			},
		},
		{
			name:           "reminder by id is returned",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminder":{"id":9,"rule_id":11,"title":"Details","description":"Full","remind_at":"2026-07-20T10:00:00Z","status":"sent","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminder, err := client.ReminderByID(ctx, chatID, 9)

				return fmt.Sprintf("%d|%d|%s", reminder.ID, *reminder.RuleID, reminder.Title), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "9|11|Details",
				requestPath:    "/api/v1/reminders/9",
				telegramHeader: "123456",
			},
		},
		{
			name:           "one time reminder is created",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminder":{"id":10,"rule_id":null,"title":"Created","description":"","remind_at":"2026-07-20T10:00:00Z","status":"new","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminder, err := client.CreateReminder(ctx, chatID, "Created", "", remindAt)

				return fmt.Sprintf("%d|%s", reminder.ID, reminder.Title), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "10|Created",
				requestPath:    "/api/v1/reminders",
				telegramHeader: "123456",
			},
		},
		{
			name:           "interval reminder is created",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminder":{"id":11,"rule_id":12,"title":"Interval","description":"","remind_at":"2026-07-20T10:00:00Z","status":"new","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminder, err := client.CreateIntervalReminder(ctx, chatID, "Interval", "", remindAt, 2)

				return fmt.Sprintf("%d|%s", reminder.ID, reminder.Title), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "11|Interval",
				requestPath:    "/api/v1/reminders",
				telegramHeader: "123456",
			},
		},
		{
			name:           "weekdays reminder is created",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminder":{"id":12,"rule_id":13,"title":"Weekdays","description":"","remind_at":"2026-07-20T10:00:00Z","status":"new","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminder, err := client.CreateWeekdaysReminder(ctx, chatID, "Weekdays", "", remindAt, 31)

				return fmt.Sprintf("%d|%s", reminder.ID, reminder.Title), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "12|Weekdays",
				requestPath:    "/api/v1/reminders",
				telegramHeader: "123456",
			},
		},
		{
			name:           "custom dates reminder is created",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminder":{"id":13,"rule_id":14,"title":"Custom","description":"","remind_at":"2026-07-20T10:00:00Z","status":"new","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminder, err := client.CreateCustomDatesReminder(
					ctx,
					chatID,
					"Custom",
					"",
					[]time.Time{holidayDate},
					10,
					30,
					time.UTC,
				)

				return fmt.Sprintf("%d|%s", reminder.ID, reminder.Title), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "13|Custom",
				requestPath:    "/api/v1/reminders",
				telegramHeader: "123456",
			},
		},
		{
			name:           "reminder is updated",
			responseStatus: http.StatusOK,
			responseBody:   `{"reminder":{"id":14,"rule_id":null,"title":"Updated","description":"Text","remind_at":"2026-07-20T10:00:00Z","status":"new","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				reminder, err := client.UpdateReminder(ctx, chatID, 14, "Updated", "Text", remindAt)

				return fmt.Sprintf("%d|%s|%s", reminder.ID, reminder.Title, reminder.Description), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "14|Updated|Text",
				requestPath:    "/api/v1/reminders/14",
				telegramHeader: "123456",
			},
		},
		{
			name:           "reminder is deleted",
			responseStatus: http.StatusOK,
			responseBody:   `{}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				err := client.DeleteReminder(ctx, chatID, 15)

				return "deleted", err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "deleted",
				requestPath:    "/api/v1/reminders/15",
				telegramHeader: "123456",
			},
		},
		{
			name:           "reminder is completed",
			responseStatus: http.StatusOK,
			responseBody:   `{}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				err := client.AcknowledgeReminder(ctx, chatID, 16, "completed")

				return "completed", err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "completed",
				requestPath:    "/api/v1/reminders/16/complete",
				telegramHeader: "123456",
			},
		},
		{
			name:           "reminder is canceled",
			responseStatus: http.StatusOK,
			responseBody:   `{}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				err := client.AcknowledgeReminder(ctx, chatID, 17, "canceled")

				return "canceled", err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "canceled",
				requestPath:    "/api/v1/reminders/17/cancel",
				telegramHeader: "123456",
			},
		},
		{
			name:           "holiday is created",
			responseStatus: http.StatusOK,
			responseBody:   `{"holiday":{"id":18,"telegram_chat_id":123456,"holiday_name":"Birthday","recipient_name":"Oleg","holiday_date":"2026-07-21T00:00:00Z","remind_at":"2026-07-20T10:00:00Z","status":"active","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				holiday, err := client.CreateHoliday(ctx, telegrammodel.HolidayRequest{
					TelegramChatID: chatID,
					HolidayName:    "Birthday",
					RecipientName:  "Oleg",
					HolidayDate:    holidayDate,
					RemindAt:       remindAt,
				})

				return fmt.Sprintf("%d|%s|%s", holiday.ID, holiday.HolidayName, holiday.RecipientName), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "18|Birthday|Oleg",
				requestPath:    "/api/v1/holidays",
				telegramHeader: "123456",
			},
		},
		{
			name:           "holiday is updated",
			responseStatus: http.StatusOK,
			responseBody:   `{"holiday":{"id":19,"telegram_chat_id":123456,"holiday_name":"Anniversary","recipient_name":"Mira","holiday_date":"2026-07-21T00:00:00Z","remind_at":"2026-07-20T10:00:00Z","status":"active","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				holiday, err := client.UpdateHoliday(ctx, telegrammodel.HolidayUpdateRequest{
					ID: 19,
					HolidayRequest: telegrammodel.HolidayRequest{
						TelegramChatID: chatID,
						HolidayName:    "Anniversary",
						RecipientName:  "Mira",
						HolidayDate:    holidayDate,
						RemindAt:       remindAt,
					},
				})

				return fmt.Sprintf("%d|%s|%s", holiday.ID, holiday.HolidayName, holiday.RecipientName), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "19|Anniversary|Mira",
				requestPath:    "/api/v1/holidays/19",
				telegramHeader: "123456",
			},
		},
		{
			name:           "holidays are returned",
			responseStatus: http.StatusOK,
			responseBody:   `{"holidays":[{"id":20,"telegram_chat_id":123456,"holiday_name":"Birthday","recipient_name":"Oleg","holiday_date":"2026-07-21T00:00:00Z","remind_at":"2026-07-20T10:00:00Z","status":"active","created_at":"2026-07-20T09:00:00Z"}]}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				holidays, err := client.FindHolidaysByChat(ctx, chatID, 4, 8)

				return fmt.Sprintf("%d|%s", len(holidays), holidays[0].HolidayName), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "1|Birthday",
				requestPath:    "/api/v1/holidays?limit=4&offset=8",
				telegramHeader: "123456",
			},
		},
		{
			name:           "holiday by id is returned",
			responseStatus: http.StatusOK,
			responseBody:   `{"holiday":{"id":21,"telegram_chat_id":123456,"holiday_name":"Birthday","recipient_name":"Oleg","holiday_date":"2026-07-21T00:00:00Z","remind_at":"2026-07-20T10:00:00Z","status":"active","created_at":"2026-07-20T09:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				holiday, err := client.HolidayByID(ctx, chatID, 21)

				return fmt.Sprintf("%d|%s", holiday.ID, holiday.HolidayName), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "21|Birthday",
				requestPath:    "/api/v1/holidays/21",
				telegramHeader: "123456",
			},
		},
		{
			name:           "holiday is deleted",
			responseStatus: http.StatusOK,
			responseBody:   `{}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				err := client.DeleteHoliday(ctx, chatID, 22)

				return "holiday deleted", err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "holiday deleted",
				requestPath:    "/api/v1/holidays/22",
				telegramHeader: "123456",
			},
		},
		{
			name:           "shopping list is created",
			responseStatus: http.StatusOK,
			responseBody:   `{"shopping_list":{"id":23,"telegram_chat_id":123456,"created_at":"2026-07-20T09:00:00Z","items":[{"id":24,"list_id":23,"text":"Milk","done":false,"position":1,"created_at":"2026-07-20T09:00:00Z"}]}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				list, err := client.CreateShoppingList(ctx, chatID, []telegrammodel.ShoppingListItem{{Text: "Milk"}})

				return fmt.Sprintf("%d|%s", list.ID, list.Items[0].Text), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "23|Milk",
				requestPath:    "/api/v1/shopping-lists",
				telegramHeader: "123456",
			},
		},
		{
			name:           "shopping lists are returned",
			responseStatus: http.StatusOK,
			responseBody:   `{"shopping_lists":[{"id":24,"telegram_chat_id":123456,"created_at":"2026-07-20T09:00:00Z","items":[{"id":25,"list_id":24,"text":"Bread","done":true,"position":1,"created_at":"2026-07-20T09:00:00Z"}]}]}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				lists, err := client.FindShoppingListsByChat(ctx, chatID, 5, 10)

				return fmt.Sprintf("%d|%s|%t", len(lists), lists[0].Items[0].Text, lists[0].Items[0].Done), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "1|Bread|true",
				requestPath:    "/api/v1/shopping-lists?limit=5&offset=10",
				telegramHeader: "123456",
			},
		},
		{
			name:           "shopping list by id is returned",
			responseStatus: http.StatusOK,
			responseBody:   `{"shopping_list":{"id":26,"telegram_chat_id":123456,"created_at":"2026-07-20T09:00:00Z","items":[{"id":27,"list_id":26,"text":"Eggs","done":false,"position":1,"created_at":"2026-07-20T09:00:00Z"}]}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				list, err := client.ShoppingListByID(ctx, chatID, 26)

				return fmt.Sprintf("%d|%s", list.ID, list.Items[0].Text), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "26|Eggs",
				requestPath:    "/api/v1/shopping-lists/26",
				telegramHeader: "123456",
			},
		},
		{
			name:           "shopping list item is toggled",
			responseStatus: http.StatusOK,
			responseBody:   `{"shopping_list":{"id":27,"telegram_chat_id":123456,"created_at":"2026-07-20T09:00:00Z","items":[{"id":28,"list_id":27,"text":"Eggs","done":true,"position":1,"created_at":"2026-07-20T09:00:00Z"}]}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				list, err := client.ToggleShoppingListItem(ctx, chatID, 28)

				return fmt.Sprintf("%d|%t", list.ID, list.Items[0].Done), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "27|true",
				requestPath:    "/api/v1/shopping-list-items/28/toggle",
				telegramHeader: "123456",
			},
		},
		{
			name:           "timezone is updated",
			responseStatus: http.StatusOK,
			responseBody:   `{"settings":{"telegram_chat_id":123456,"language":"ru","timezone":"Europe/Moscow","intrusive_enabled":true,"intrusive_interval_minutes":30,"updated_at":"2026-07-20T10:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				err := client.UpdateUserTimezone(ctx, chatID, "Europe/Moscow")

				return "timezone updated", err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "timezone updated",
				requestPath:    "/api/v1/settings",
				telegramHeader: "123456",
			},
		},
		{
			name:           "intrusive settings are updated",
			responseStatus: http.StatusOK,
			responseBody:   `{"settings":{"telegram_chat_id":123456,"language":"ru","timezone":"Europe/Moscow","intrusive_enabled":false,"intrusive_interval_minutes":45,"updated_at":"2026-07-20T10:00:00Z"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				err := client.UpdateIntrusiveSettings(ctx, chatID, false, 45*time.Minute)

				return "intrusive updated", err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:         "intrusive updated",
				requestPath:    "/api/v1/settings",
				telegramHeader: "123456",
			},
		},
		{
			name:           "timezones are returned without telegram header",
			responseStatus: http.StatusOK,
			responseBody:   `{"timezones":[{"city":"Москва","timezone":"Europe/Moscow"}]}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				timezones, err := client.Timezones(ctx)

				return fmt.Sprintf("%d|%s", len(timezones), timezones[0].Timezone), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:      "1|Europe/Moscow",
				requestPath: "/api/v1/timezones",
			},
		},
		{
			name:           "backend error response is returned",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error":{"code":500,"message":"internal server error"}}`,
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				stats, err := client.Dashboard(ctx, chatID)

				return fmt.Sprintf("%d/%d/%d", stats.Completed, stats.Pending, stats.Skipped), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:          "0/0/0",
				requestPath:     "/api/v1/dashboard",
				telegramHeader:  "123456",
				expectedErrText: "load dashboard from backend: backend request failed with status 500: internal server error",
			},
		},
		{
			name: "unsupported acknowledge status returns error",
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				err := client.AcknowledgeReminder(ctx, chatID, 18, "sent")

				return "unsupported", err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:          "unsupported",
				expectedErrText: "unsupported delivered reminder status: sent",
			},
		},
		{
			name: "nil backend client returns error",
			action: func(ctx context.Context, client *BackendClient) (string, error) {
				stats, err := client.Dashboard(ctx, chatID)

				return fmt.Sprintf("%d/%d/%d", stats.Completed, stats.Pending, stats.Skipped), err
			},
			expected: struct {
				result          string
				requestPath     string
				telegramHeader  string
				expectedErrText string
			}{
				result:          "0/0/0",
				expectedErrText: "load dashboard from backend: backend client is missing",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var (
				requestPath    string
				telegramHeader string
				client         *BackendClient
			)

			if test.responseStatus != 0 {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
					requestPath = request.URL.RequestURI()
					telegramHeader = request.Header.Get(apphttp.TelegramChatIDHeader)

					w.WriteHeader(test.responseStatus)
					_, _ = w.Write([]byte(test.responseBody))
				}))
				t.Cleanup(server.Close)

				var err error

				client, err = NewBackendClient(server.URL+test.baseURLSuffix, time.Second)
				require.NoError(t, err)
			}

			result, err := test.action(context.Background(), client)

			if test.expected.expectedErrText != "" {
				require.EqualErrorf(t, err, test.expected.expectedErrText, "unexpected backend client error")
				require.Equal(t, test.expected.result, result)
				require.Equal(t, test.expected.requestPath, requestPath)
				require.Equal(t, test.expected.telegramHeader, telegramHeader)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected.result, result)
			require.Equal(t, test.expected.requestPath, requestPath)
			require.Equal(t, test.expected.telegramHeader, telegramHeader)
		})
	}
}
