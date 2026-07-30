package router_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/service/mocks"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/router"
	"go.uber.org/mock/gomock"
)

func TestQueryReminders(t *testing.T) {
	t.Parallel()

	const (
		telegramChatID  = int64(123456)
		applicationJSON = "application/json"
	)

	tests := []struct {
		name           string
		contentType    string
		body           string
		expectQuery    bool
		expectedStatus int
	}{
		{
			name:           "query returns reminders filtered by statuses",
			contentType:    applicationJSON,
			body:           `{"statuses":["completed","canceled"],"limit":2,"offset":4}`,
			expectQuery:    true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "query requires json content type",
			body:           `{"statuses":["completed"]}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "query rejects unsupported content type",
			contentType:    "text/plain",
			body:           `{"statuses":["completed"]}`,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "query rejects unsupported status",
			contentType:    applicationJSON,
			body:           `{"statuses":["unknown"]}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "query rejects duplicate status",
			contentType:    applicationJSON,
			body:           `{"statuses":["failed","failed"]}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "query requires at least one status",
			contentType:    applicationJSON,
			body:           `{"statuses":[]}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controller := gomock.NewController(t)

			repository := mocks.NewMockReminderCommandRepository(controller)
			if test.expectQuery {
				repository.EXPECT().
					FindRemindersByChat(
						gomock.Any(),
						telegramChatID,
						[]database.ReminderStatus{
							database.ReminderStatusCompleted,
							database.ReminderStatusCanceled,
						},
						2,
						4,
					).
					Return([]database.ReminderView{}, nil)
			}

			reminderService := service.NewReminderService(repository, nil)
			handler := router.Create(reminderService)

			request := httptest.NewRequestWithContext(
				context.Background(),
				apphttp.MethodQuery,
				"/api/v1/reminders",
				bytes.NewBufferString(test.body),
			)
			request.Header.Set(apphttp.TelegramChatIDHeader, "123456")

			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf(
					"unexpected status: got %d, want %d; body: %s",
					response.Code,
					test.expectedStatus,
					response.Body,
				)
			}

			if response.Header().Get("Accept-Query") != applicationJSON {
				t.Errorf("unexpected Accept-Query header: %q", response.Header().Get("Accept-Query"))
			}
		})
	}
}
