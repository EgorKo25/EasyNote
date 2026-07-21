package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
)

const apiV1Prefix = "/api/v1"

var errBackendClientMissing = errors.New("backend client is missing")

type BackendClient struct {
	baseURL string
	client  *http.Client
}

func NewBackendClient(baseURL string, timeout time.Duration) *BackendClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}

	return &BackendClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *BackendClient) Dashboard(ctx context.Context, telegramChatID int64) (database.ReminderStats, error) {
	var response backendDashboardResponse
	if err := c.get(ctx, telegramChatID, apiV1Prefix+"/dashboard", &response); err != nil {
		return database.ReminderStats{}, fmt.Errorf("load dashboard from backend: %w", err)
	}

	return database.ReminderStats{
		Completed: response.Stats.Completed,
		Pending:   response.Stats.Pending,
		Skipped:   response.Stats.Skipped,
	}, nil
}

func (c *BackendClient) Settings(ctx context.Context, telegramChatID int64) (database.UserSettings, error) {
	var response backendSettingsResponse
	if err := c.get(ctx, telegramChatID, apiV1Prefix+"/settings", &response); err != nil {
		return database.UserSettings{}, fmt.Errorf("load settings from backend: %w", err)
	}

	return database.UserSettings{
		TelegramChatID:    response.Settings.TelegramChatID,
		Language:          response.Settings.Language,
		Timezone:          response.Settings.Timezone,
		IntrusiveEnabled:  response.Settings.IntrusiveEnabled,
		IntrusiveInterval: time.Duration(response.Settings.IntrusiveIntervalMinutes) * time.Minute,
		UpdatedAt:         response.Settings.UpdatedAt,
	}, nil
}

func (c *BackendClient) get(ctx context.Context, telegramChatID int64, path string, response any) error {
	if c == nil {
		return errBackendClientMissing
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create backend request: %w", err)
	}

	request.Header.Set(apphttp.TelegramChatIDHeader, strconv.FormatInt(telegramChatID, 10))
	request.Header.Set("Accept", "application/json")

	httpResponse, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("send backend request: %w", err)
	}
	defer func() { _ = httpResponse.Body.Close() }()

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		var errorResponse backendErrorResponse
		if err = json.NewDecoder(httpResponse.Body).Decode(&errorResponse); err != nil {
			return fmt.Errorf("backend request failed with status %d", httpResponse.StatusCode)
		}

		message := errorResponse.Error.Message
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}

		return fmt.Errorf("backend request failed with status %d: %s", httpResponse.StatusCode, message)
	}

	if err = json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
		return fmt.Errorf("decode backend response: %w", err)
	}

	return nil
}

type backendDashboardResponse struct {
	Stats backendReminderStats `json:"stats"`
}

type backendReminderStats struct {
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
	Skipped   int `json:"skipped"`
}

type backendSettingsResponse struct {
	Settings backendUserSettings `json:"settings"`
}

type backendUserSettings struct {
	TelegramChatID           int64     `json:"telegram_chat_id"`
	Language                 string    `json:"language"`
	Timezone                 string    `json:"timezone"`
	IntrusiveEnabled         bool      `json:"intrusive_enabled"`
	IntrusiveIntervalMinutes int       `json:"intrusive_interval_minutes"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type backendErrorResponse struct {
	Error backendError `json:"error"`
}

type backendError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
