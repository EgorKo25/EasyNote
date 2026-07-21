package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/stretchr/testify/require"
)

func TestBackendClientSendsTelegramChatRequests(t *testing.T) {
	t.Parallel()

	chatID := int64(123456)

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

				return fmt.Sprintf("%s|%t|%s", settings.Timezone, settings.IntrusiveEnabled, settings.IntrusiveInterval), err
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

			var requestPath string
			var telegramHeader string
			var client *BackendClient

			if test.responseStatus != 0 {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
					requestPath = request.URL.Path
					telegramHeader = request.Header.Get(apphttp.TelegramChatIDHeader)

					w.WriteHeader(test.responseStatus)
					_, _ = w.Write([]byte(test.responseBody))
				}))
				t.Cleanup(server.Close)

				client = NewBackendClient(server.URL+test.baseURLSuffix, time.Second)
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
