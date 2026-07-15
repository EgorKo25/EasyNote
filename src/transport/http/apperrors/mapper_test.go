package apperrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/EgorKo25/EasyNote/src/transport/http/commands"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
)

func TestMapErrorResponse(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorResponse
	}{
		{
			name: "maps missing telegram chat id to bad request",
			err:  middleware.ErrTelegramChatIDMissing,
			expected: newErrorResponse(
				http.StatusBadRequest,
				middleware.ErrTelegramChatIDMissing.Error(),
			),
		},
		{
			name: "maps invalid request to bad request",
			err:  commands.ErrInvalidRequest,
			expected: newErrorResponse(
				http.StatusBadRequest,
				commands.ErrInvalidRequest.Error(),
			),
		},
		{
			name: "maps invalid pagination offset to bad request",
			err:  commands.ErrInvalidOffset,
			expected: newErrorResponse(
				http.StatusBadRequest,
				commands.ErrInvalidOffset.Error(),
			),
		},
		{
			name: "maps invalid pagination limit to bad request",
			err:  commands.ErrInvalidLimit,
			expected: newErrorResponse(
				http.StatusBadRequest,
				commands.ErrInvalidLimit.Error(),
			),
		},
		{
			name: "maps wrapped reminder repeat error to bad request",
			err:  fmt.Errorf("parse request: %w", commands.ErrReminderRepeatTypeUnsupported),
			expected: newErrorResponse(
				http.StatusBadRequest,
				fmt.Errorf("parse request: %w", commands.ErrReminderRepeatTypeUnsupported).Error(),
			),
		},
		{
			name: "maps service weekdays mask validation error to bad request",
			err:  service.ErrWeekdaysMaskEmpty,
			expected: newErrorResponse(
				http.StatusBadRequest,
				service.ErrWeekdaysMaskEmpty.Error(),
			),
		},
		{
			name: "maps service access error to forbidden",
			err:  service.ErrAccessDenied,
			expected: newErrorResponse(
				http.StatusForbidden,
				service.ErrAccessDenied.Error(),
			),
		},
		{
			name: "maps service not found error to not found",
			err:  service.ErrReminderNotFound,
			expected: newErrorResponse(
				http.StatusNotFound,
				service.ErrReminderNotFound.Error(),
			),
		},
		{
			name: "maps database not found error to not found",
			err:  database.ErrShoppingListNotFound,
			expected: newErrorResponse(
				http.StatusNotFound,
				database.ErrShoppingListNotFound.Error(),
			),
		},
		{
			name: "maps unknown storage error to internal server error without leaking message",
			err:  errors.New("postgres password leaked"),
			expected: newErrorResponse(
				http.StatusInternalServerError,
				"internal server error",
			),
		},
		{
			name: "maps unknown wrapped error to internal server error without leaking message",
			err:  fmt.Errorf("background operation failed: %w", errors.New("secret token leaked")),
			expected: newErrorResponse(
				http.StatusInternalServerError,
				"internal server error",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := mapErrorResponse(test.err)

			require.Equal(t, test.expected, actual)
		})
	}
}

func TestWriteError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		err      error
		expected ErrorResponse
	}{
		{
			name: "writes middleware bad request response",
			err:  middleware.ErrTelegramChatIDInvalid,
			expected: newErrorResponse(
				http.StatusBadRequest,
				middleware.ErrTelegramChatIDInvalid.Error(),
			),
		},
		{
			name: "writes command bad request response",
			err:  commands.ErrReminderRepeatDateInvalid,
			expected: newErrorResponse(
				http.StatusBadRequest,
				commands.ErrReminderRepeatDateInvalid.Error(),
			),
		},
		{
			name: "writes forbidden response",
			err:  service.ErrAccessDenied,
			expected: newErrorResponse(
				http.StatusForbidden,
				service.ErrAccessDenied.Error(),
			),
		},
		{
			name: "writes service not found response",
			err:  service.ErrReminderNotFound,
			expected: newErrorResponse(
				http.StatusNotFound,
				service.ErrReminderNotFound.Error(),
			),
		},
		{
			name: "writes database not found response",
			err:  database.ErrReminderNotFound,
			expected: newErrorResponse(
				http.StatusNotFound,
				database.ErrReminderNotFound.Error(),
			),
		},
		{
			name: "writes unknown error response without leaking message",
			err:  errors.New("private network failure"),
			expected: newErrorResponse(
				http.StatusInternalServerError,
				"internal server error",
			),
		},
		{
			name: "writes wrapped unknown error response without leaking message",
			err:  fmt.Errorf("request failed: %w", errors.New("private payload")),
			expected: newErrorResponse(
				http.StatusInternalServerError,
				"internal server error",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)

			WriteError(ctx, test.err)

			var actual ErrorResponse
			err := json.NewDecoder(recorder.Body).Decode(&actual)

			require.NoError(t, err)
			require.Equal(t, test.expected.Error.Code, recorder.Code)
			require.Equal(t, test.expected, actual)
		})
	}
}
