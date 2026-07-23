package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	telegrammodel "github.com/EgorKo25/EasyNote/src/telegram/model"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
)

const apiV1Prefix = "/api/v1"

var errBackendClientMissing = errors.New("backend client is missing")
var errBackendRequestFailed = errors.New("backend request failed")
var ErrBackendBaseURLMissing = errors.New("telegram backend base url is empty")

type BackendClient struct {
	baseURL string
	client  *http.Client
}

func NewBackendClient(baseURL string, timeout time.Duration) (*BackendClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, ErrBackendBaseURLMissing
	}

	return &BackendClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *BackendClient) Dashboard(ctx context.Context, telegramChatID int64) (telegrammodel.ReminderStats, error) {
	stats, err := c.ReminderStatsByChat(ctx, telegramChatID)
	if err != nil {
		return telegrammodel.ReminderStats{}, fmt.Errorf("load dashboard from backend: %w", err)
	}

	return stats, nil
}

func (c *BackendClient) Settings(ctx context.Context, telegramChatID int64) (telegrammodel.UserSettings, error) {
	settings, err := c.UserSettings(ctx, telegramChatID)
	if err != nil {
		return telegrammodel.UserSettings{}, fmt.Errorf("load settings from backend: %w", err)
	}

	return settings, nil
}

func (c *BackendClient) ReminderStatsByChat(
	ctx context.Context,
	telegramChatID int64,
) (telegrammodel.ReminderStats, error) {
	var response backendDashboardResponse
	if err := c.get(ctx, telegramChatID, apiV1Prefix+"/dashboard", &response); err != nil {
		return telegrammodel.ReminderStats{}, err
	}

	return telegrammodel.ReminderStats{
		Completed: response.Stats.Completed,
		Pending:   response.Stats.Pending,
		Skipped:   response.Stats.Skipped,
	}, nil
}

func (c *BackendClient) UserSettings(ctx context.Context, telegramChatID int64) (telegrammodel.UserSettings, error) {
	var response backendSettingsResponse
	if err := c.get(ctx, telegramChatID, apiV1Prefix+"/settings", &response); err != nil {
		return telegrammodel.UserSettings{}, err
	}

	return response.Settings.telegramSettings(), nil
}

func (c *BackendClient) UpdateUserTimezone(ctx context.Context, telegramChatID int64, timezone string) error {
	_, err := c.updateSettings(ctx, telegramChatID, backendUpdateSettingsRequest{Timezone: &timezone})
	if err != nil {
		return fmt.Errorf("update user timezone through backend: %w", err)
	}

	return nil
}

func (c *BackendClient) UpdateIntrusiveSettings(
	ctx context.Context,
	telegramChatID int64,
	enabled bool,
	interval time.Duration,
) error {
	minutes := int(interval / time.Minute)

	_, err := c.updateSettings(ctx, telegramChatID, backendUpdateSettingsRequest{
		IntrusiveEnabled:         &enabled,
		IntrusiveIntervalMinutes: &minutes,
	})
	if err != nil {
		return fmt.Errorf("update intrusive settings through backend: %w", err)
	}

	return nil
}

func (c *BackendClient) FindRemindersByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]telegrammodel.ReminderView, error) {
	var response backendReminderListResponse
	if err := c.get(
		ctx,
		telegramChatID,
		pathWithPagination(apiV1Prefix+"/reminders", limit, offset),
		&response,
	); err != nil {
		return nil, fmt.Errorf("load reminders through backend: %w", err)
	}

	return mapBackendReminders(response.Reminders), nil
}

func (c *BackendClient) FindReminderHistoryByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]telegrammodel.ReminderView, error) {
	var response backendReminderListResponse
	if err := c.get(
		ctx,
		telegramChatID,
		pathWithPagination(apiV1Prefix+"/reminders/history", limit, offset),
		&response,
	); err != nil {
		return nil, fmt.Errorf("load reminder history through backend: %w", err)
	}

	return mapBackendReminders(response.Reminders), nil
}

func (c *BackendClient) ReminderByID(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
) (telegrammodel.ReminderView, error) {
	var response backendReminderResponse
	if err := c.get(
		ctx,
		telegramChatID,
		apiV1Prefix+"/reminders/"+strconv.FormatInt(reminderID, 10),
		&response,
	); err != nil {
		return telegrammodel.ReminderView{}, fmt.Errorf("load reminder through backend: %w", err)
	}

	return response.Reminder.telegramReminder(), nil
}

func (c *BackendClient) CreateReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
) (telegrammodel.ReminderView, error) {
	return c.createReminder(ctx, telegramChatID, backendReminderRequest{
		Title:       title,
		Description: description,
		RemindAt:    remindAt,
	})
}

func (c *BackendClient) CreateIntervalReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
	intervalDays int,
) (telegrammodel.ReminderView, error) {
	return c.createReminder(ctx, telegramChatID, backendReminderRequest{
		Title:       title,
		Description: description,
		RemindAt:    remindAt,
		Repeat: &backendReminderRepeatRequest{
			Type:         string(telegrammodel.ReminderRuleTypeInterval),
			IntervalDays: &intervalDays,
		},
	})
}

func (c *BackendClient) CreateWeekdaysReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
	weekdaysMask int,
) (telegrammodel.ReminderView, error) {
	return c.createReminder(ctx, telegramChatID, backendReminderRequest{
		Title:       title,
		Description: description,
		RemindAt:    remindAt,
		Repeat: &backendReminderRepeatRequest{
			Type:         string(telegrammodel.ReminderRuleTypeWeekdays),
			WeekdaysMask: &weekdaysMask,
		},
	})
}

func (c *BackendClient) CreateCustomDatesReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	selectedDates []time.Time,
	hour int,
	minute int,
	location *time.Location,
) (telegrammodel.ReminderView, error) {
	if location == nil {
		location = time.UTC
	}

	timezone := location.String()

	dates := make([]string, 0, len(selectedDates))
	for _, date := range selectedDates {
		dates = append(dates, date.Format(time.DateOnly))
	}

	remindAt := time.Time{}
	if len(selectedDates) > 0 {
		remindAt = time.Date(
			selectedDates[0].Year(),
			selectedDates[0].Month(),
			selectedDates[0].Day(),
			hour,
			minute,
			0,
			0,
			location,
		)
	}

	return c.createReminder(ctx, telegramChatID, backendReminderRequest{
		Title:       title,
		Description: description,
		RemindAt:    remindAt,
		Repeat: &backendReminderRepeatRequest{
			Type:     string(telegrammodel.ReminderRuleTypeCustomDates),
			Dates:    dates,
			Hour:     &hour,
			Minute:   &minute,
			Timezone: timezone,
		},
	})
}

func (c *BackendClient) UpdateReminder(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
	title string,
	description string,
	remindAt time.Time,
) (telegrammodel.ReminderView, error) {
	var response backendReminderResponse
	if err := c.patch(
		ctx,
		telegramChatID,
		apiV1Prefix+"/reminders/"+strconv.FormatInt(reminderID, 10),
		backendReminderRequest{
			Title:       title,
			Description: description,
			RemindAt:    remindAt,
		},
		&response,
	); err != nil {
		return telegrammodel.ReminderView{}, fmt.Errorf("update reminder through backend: %w", err)
	}

	return response.Reminder.telegramReminder(), nil
}

func (c *BackendClient) DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error {
	if err := c.delete(
		ctx,
		telegramChatID,
		apiV1Prefix+"/reminders/"+strconv.FormatInt(reminderID, 10),
		nil,
	); err != nil {
		return fmt.Errorf("delete reminder through backend: %w", err)
	}

	return nil
}

func (c *BackendClient) AcknowledgeReminder(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
	status telegrammodel.ReminderStatus,
) error {
	var path string

	switch status {
	case telegrammodel.ReminderStatusCompleted:
		path = apiV1Prefix + "/reminders/" + strconv.FormatInt(reminderID, 10) + "/complete"
	case telegrammodel.ReminderStatusCanceled:
		path = apiV1Prefix + "/reminders/" + strconv.FormatInt(reminderID, 10) + "/cancel"
	default:
		return fmt.Errorf("%w: %s", errUnsupportedDeliveredReminderStatus, status)
	}

	if err := c.post(ctx, telegramChatID, path, nil, nil); err != nil {
		return fmt.Errorf("acknowledge reminder through backend: %w", err)
	}

	return nil
}

func (c *BackendClient) CreateHoliday(
	ctx context.Context,
	request telegrammodel.HolidayRequest,
) (telegrammodel.HolidayView, error) {
	var response backendHolidayResponse
	if err := c.post(ctx, request.TelegramChatID, apiV1Prefix+"/holidays", backendHolidayRequest{
		HolidayName:   request.HolidayName,
		RecipientName: request.RecipientName,
		HolidayDate:   request.HolidayDate.Format(time.DateOnly),
		RemindAt:      request.RemindAt,
	}, &response); err != nil {
		return telegrammodel.HolidayView{}, fmt.Errorf("create holiday through backend: %w", err)
	}

	return response.Holiday.telegramHoliday(), nil
}

func (c *BackendClient) UpdateHoliday(
	ctx context.Context,
	request telegrammodel.HolidayUpdateRequest,
) (telegrammodel.HolidayView, error) {
	var response backendHolidayResponse
	if err := c.patch(
		ctx,
		request.TelegramChatID,
		apiV1Prefix+"/holidays/"+strconv.FormatInt(request.ID, 10),
		backendHolidayRequest{
			HolidayName:   request.HolidayName,
			RecipientName: request.RecipientName,
			HolidayDate:   request.HolidayDate.Format(time.DateOnly),
			RemindAt:      request.RemindAt,
		},
		&response,
	); err != nil {
		return telegrammodel.HolidayView{}, fmt.Errorf("update holiday through backend: %w", err)
	}

	return response.Holiday.telegramHoliday(), nil
}

func (c *BackendClient) FindHolidaysByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]telegrammodel.HolidayView, error) {
	var response backendHolidayListResponse
	if err := c.get(
		ctx,
		telegramChatID,
		pathWithPagination(apiV1Prefix+"/holidays", limit, offset),
		&response,
	); err != nil {
		return nil, fmt.Errorf("load holidays through backend: %w", err)
	}

	return mapBackendHolidays(response.Holidays), nil
}

func (c *BackendClient) HolidayByID(
	ctx context.Context,
	telegramChatID int64,
	holidayID int64,
) (telegrammodel.HolidayView, error) {
	var response backendHolidayResponse
	if err := c.get(
		ctx,
		telegramChatID,
		apiV1Prefix+"/holidays/"+strconv.FormatInt(holidayID, 10),
		&response,
	); err != nil {
		return telegrammodel.HolidayView{}, fmt.Errorf("load holiday through backend: %w", err)
	}

	return response.Holiday.telegramHoliday(), nil
}

func (c *BackendClient) DeleteHoliday(ctx context.Context, telegramChatID int64, holidayID int64) error {
	if err := c.delete(
		ctx,
		telegramChatID,
		apiV1Prefix+"/holidays/"+strconv.FormatInt(holidayID, 10),
		nil,
	); err != nil {
		return fmt.Errorf("delete holiday through backend: %w", err)
	}

	return nil
}

func (c *BackendClient) CreateShoppingList(
	ctx context.Context,
	telegramChatID int64,
	items []telegrammodel.ShoppingListItem,
) (telegrammodel.ShoppingListView, error) {
	requestItems := make([]backendShoppingListItemRequest, 0, len(items))
	for _, item := range items {
		requestItems = append(requestItems, backendShoppingListItemRequest{Text: item.Text})
	}

	var response backendShoppingListResponse
	if err := c.post(ctx, telegramChatID, apiV1Prefix+"/shopping-lists", backendShoppingListRequest{
		Items: requestItems,
	}, &response); err != nil {
		return telegrammodel.ShoppingListView{}, fmt.Errorf("create shopping list through backend: %w", err)
	}

	return response.ShoppingList.telegramShoppingList(), nil
}

func (c *BackendClient) FindShoppingListsByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]telegrammodel.ShoppingListView, error) {
	var response backendShoppingListsResponse
	if err := c.get(
		ctx,
		telegramChatID,
		pathWithPagination(apiV1Prefix+"/shopping-lists", limit, offset),
		&response,
	); err != nil {
		return nil, fmt.Errorf("load shopping lists through backend: %w", err)
	}

	return mapBackendShoppingLists(response.ShoppingLists), nil
}

func (c *BackendClient) ShoppingListByID(
	ctx context.Context,
	telegramChatID int64,
	listID int64,
) (telegrammodel.ShoppingListView, error) {
	var response backendShoppingListResponse
	if err := c.get(
		ctx,
		telegramChatID,
		apiV1Prefix+"/shopping-lists/"+strconv.FormatInt(listID, 10),
		&response,
	); err != nil {
		return telegrammodel.ShoppingListView{}, fmt.Errorf("load shopping list through backend: %w", err)
	}

	return response.ShoppingList.telegramShoppingList(), nil
}

func (c *BackendClient) ToggleShoppingListItem(
	ctx context.Context,
	telegramChatID int64,
	itemID int64,
) (telegrammodel.ShoppingListView, error) {
	var response backendShoppingListResponse
	if err := c.post(
		ctx,
		telegramChatID,
		apiV1Prefix+"/shopping-list-items/"+strconv.FormatInt(itemID, 10)+"/toggle",
		nil,
		&response,
	); err != nil {
		return telegrammodel.ShoppingListView{}, fmt.Errorf("toggle shopping list item through backend: %w", err)
	}

	return response.ShoppingList.telegramShoppingList(), nil
}

func (c *BackendClient) Timezones(ctx context.Context) ([]BackendTimezone, error) {
	var response backendTimezoneListResponse
	if err := c.get(ctx, 0, apiV1Prefix+"/timezones", &response); err != nil {
		return nil, fmt.Errorf("load timezones through backend: %w", err)
	}

	return response.Timezones, nil
}

func (c *BackendClient) createReminder(
	ctx context.Context,
	telegramChatID int64,
	request backendReminderRequest,
) (telegrammodel.ReminderView, error) {
	var response backendReminderResponse
	if err := c.post(ctx, telegramChatID, apiV1Prefix+"/reminders", request, &response); err != nil {
		return telegrammodel.ReminderView{}, fmt.Errorf("create reminder through backend: %w", err)
	}

	return response.Reminder.telegramReminder(), nil
}

func (c *BackendClient) updateSettings(
	ctx context.Context,
	telegramChatID int64,
	request backendUpdateSettingsRequest,
) (telegrammodel.UserSettings, error) {
	var response backendSettingsResponse
	if err := c.patch(ctx, telegramChatID, apiV1Prefix+"/settings", request, &response); err != nil {
		return telegrammodel.UserSettings{}, err
	}

	return response.Settings.telegramSettings(), nil
}

func (c *BackendClient) get(ctx context.Context, telegramChatID int64, path string, response any) error {
	return c.do(ctx, telegramChatID, http.MethodGet, path, nil, response)
}

func (c *BackendClient) post(ctx context.Context, telegramChatID int64, path string, body any, response any) error {
	return c.do(ctx, telegramChatID, http.MethodPost, path, body, response)
}

func (c *BackendClient) patch(ctx context.Context, telegramChatID int64, path string, body any, response any) error {
	return c.do(ctx, telegramChatID, http.MethodPatch, path, body, response)
}

func (c *BackendClient) delete(ctx context.Context, telegramChatID int64, path string, response any) error {
	return c.do(ctx, telegramChatID, http.MethodDelete, path, nil, response)
}

func (c *BackendClient) do(
	ctx context.Context,
	telegramChatID int64,
	method string,
	path string,
	body any,
	response any,
) error {
	httpResponse, err := c.send(ctx, telegramChatID, method, path, body)
	if err != nil {
		return err
	}

	defer func() { _ = httpResponse.Body.Close() }()

	if err = decodeBackendError(httpResponse); err != nil {
		return err
	}

	if response == nil {
		return nil
	}

	if err = json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
		return fmt.Errorf("decode backend response: %w", err)
	}

	return nil
}

func (c *BackendClient) send(
	ctx context.Context,
	telegramChatID int64,
	method string,
	path string,
	body any,
) (*http.Response, error) {
	if c == nil {
		return nil, errBackendClientMissing
	}

	request, err := c.newRequest(ctx, telegramChatID, method, path, body)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send backend request: %w", err)
	}

	return httpResponse, nil
}

func (c *BackendClient) newRequest(
	ctx context.Context,
	telegramChatID int64,
	method string,
	path string,
	body any,
) (*http.Request, error) {
	requestBody, err := backendRequestBody(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create backend request: %w", err)
	}

	if telegramChatID != 0 {
		request.Header.Set(apphttp.TelegramChatIDHeader, strconv.FormatInt(telegramChatID, 10))
	}

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	request.Header.Set("Accept", "application/json")

	return request, nil
}

func backendRequestBody(body any) (*bytes.Reader, error) {
	if body == nil {
		return bytes.NewReader(nil), nil
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode backend request: %w", err)
	}

	return bytes.NewReader(data), nil
}

func decodeBackendError(httpResponse *http.Response) error {
	if httpResponse.StatusCode >= http.StatusOK && httpResponse.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	var errorResponse backendErrorResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&errorResponse); err != nil {
		return fmt.Errorf("%w with status %d", errBackendRequestFailed, httpResponse.StatusCode)
	}

	message := errorResponse.Error.Message
	if message == "" {
		message = http.StatusText(httpResponse.StatusCode)
	}

	return fmt.Errorf("%w with status %d: %s", errBackendRequestFailed, httpResponse.StatusCode, message)
}

func pathWithPagination(path string, limit int, offset int) string {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))

	return path + "?" + query.Encode()
}

func mapBackendReminders(reminders []backendReminder) []telegrammodel.ReminderView {
	result := make([]telegrammodel.ReminderView, 0, len(reminders))
	for _, reminder := range reminders {
		result = append(result, reminder.telegramReminder())
	}

	return result
}

func mapBackendHolidays(holidays []backendHoliday) []telegrammodel.HolidayView {
	result := make([]telegrammodel.HolidayView, 0, len(holidays))
	for _, holiday := range holidays {
		result = append(result, holiday.telegramHoliday())
	}

	return result
}

func mapBackendShoppingLists(lists []backendShoppingList) []telegrammodel.ShoppingListView {
	result := make([]telegrammodel.ShoppingListView, 0, len(lists))
	for _, list := range lists {
		result = append(result, list.telegramShoppingList())
	}

	return result
}

type backendReminderRequest struct {
	Title       string                        `json:"title"`
	Description string                        `json:"description"`
	RemindAt    time.Time                     `json:"remind_at"`
	Repeat      *backendReminderRepeatRequest `json:"repeat,omitempty"`
}

type backendReminderRepeatRequest struct {
	Type         string   `json:"type"`
	IntervalDays *int     `json:"interval_days,omitempty"`
	WeekdaysMask *int     `json:"weekdays_mask,omitempty"`
	Dates        []string `json:"dates,omitempty"`
	Hour         *int     `json:"hour,omitempty"`
	Minute       *int     `json:"minute,omitempty"`
	Timezone     string   `json:"timezone,omitempty"`
}

type backendHolidayRequest struct {
	HolidayName   string    `json:"holiday_name"`
	RecipientName string    `json:"recipient_name"`
	HolidayDate   string    `json:"holiday_date"`
	RemindAt      time.Time `json:"remind_at"`
}

type backendShoppingListRequest struct {
	Items []backendShoppingListItemRequest `json:"items"`
}

type backendShoppingListItemRequest struct {
	Text string `json:"text"`
}

type backendUpdateSettingsRequest struct {
	Timezone                 *string `json:"timezone,omitempty"`
	IntrusiveEnabled         *bool   `json:"intrusive_enabled,omitempty"`
	IntrusiveIntervalMinutes *int    `json:"intrusive_interval_minutes,omitempty"`
}

type backendDashboardResponse struct {
	Stats backendReminderStats `json:"stats"`
}

type backendReminderStats struct {
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
	Skipped   int `json:"skipped"`
}

type backendReminderResponse struct {
	Reminder backendReminder `json:"reminder"`
}

type backendReminderListResponse struct {
	Reminders []backendReminder `json:"reminders"`
}

type backendReminder struct {
	ID          int64                        `json:"id"`
	RuleID      *int64                       `json:"rule_id"`
	Title       string                       `json:"title"`
	Description string                       `json:"description"`
	RemindAt    time.Time                    `json:"remind_at"`
	Status      telegrammodel.ReminderStatus `json:"status"`
	CreatedAt   time.Time                    `json:"created_at"`
}

func (r backendReminder) telegramReminder() telegrammodel.ReminderView {
	return telegrammodel.ReminderView{
		ID:          r.ID,
		RuleID:      r.RuleID,
		Title:       r.Title,
		Description: r.Description,
		RemindAt:    r.RemindAt,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt,
	}
}

type backendHolidayResponse struct {
	Holiday backendHoliday `json:"holiday"`
}

type backendHolidayListResponse struct {
	Holidays []backendHoliday `json:"holidays"`
}

type backendHoliday struct {
	ID             int64                       `json:"id"`
	TelegramChatID int64                       `json:"telegram_chat_id"`
	HolidayName    string                      `json:"holiday_name"`
	RecipientName  string                      `json:"recipient_name"`
	HolidayDate    time.Time                   `json:"holiday_date"`
	RemindAt       time.Time                   `json:"remind_at"`
	Status         telegrammodel.HolidayStatus `json:"status"`
	CreatedAt      time.Time                   `json:"created_at"`
}

func (h backendHoliday) telegramHoliday() telegrammodel.HolidayView {
	return telegrammodel.HolidayView{
		ID:             h.ID,
		TelegramChatID: h.TelegramChatID,
		HolidayName:    h.HolidayName,
		RecipientName:  h.RecipientName,
		HolidayDate:    h.HolidayDate,
		RemindAt:       h.RemindAt,
		Status:         h.Status,
		CreatedAt:      h.CreatedAt,
	}
}

type backendShoppingListResponse struct {
	ShoppingList backendShoppingList `json:"shopping_list"`
}

type backendShoppingListsResponse struct {
	ShoppingLists []backendShoppingList `json:"shopping_lists"`
}

type backendShoppingList struct {
	ID             int64                     `json:"id"`
	TelegramChatID int64                     `json:"telegram_chat_id"`
	CreatedAt      time.Time                 `json:"created_at"`
	Items          []backendShoppingListItem `json:"items"`
}

func (l backendShoppingList) telegramShoppingList() telegrammodel.ShoppingListView {
	return telegrammodel.ShoppingListView{
		ID:             l.ID,
		TelegramChatID: l.TelegramChatID,
		CreatedAt:      l.CreatedAt,
		Items:          mapBackendShoppingListItems(l.Items),
	}
}

type backendShoppingListItem struct {
	ID        int64     `json:"id"`
	ListID    int64     `json:"list_id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

func mapBackendShoppingListItems(items []backendShoppingListItem) []telegrammodel.ShoppingListItem {
	result := make([]telegrammodel.ShoppingListItem, 0, len(items))
	for _, item := range items {
		result = append(result, telegrammodel.ShoppingListItem{
			ID:        item.ID,
			ListID:    item.ListID,
			Text:      item.Text,
			Done:      item.Done,
			Position:  item.Position,
			CreatedAt: item.CreatedAt,
		})
	}

	return result
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

func (s backendUserSettings) telegramSettings() telegrammodel.UserSettings {
	return telegrammodel.UserSettings{
		TelegramChatID:    s.TelegramChatID,
		Language:          s.Language,
		Timezone:          s.Timezone,
		IntrusiveEnabled:  s.IntrusiveEnabled,
		IntrusiveInterval: time.Duration(s.IntrusiveIntervalMinutes) * time.Minute,
		UpdatedAt:         s.UpdatedAt,
	}
}

type backendTimezoneListResponse struct {
	Timezones []BackendTimezone `json:"timezones"`
}

type BackendTimezone struct {
	City     string `json:"city"`
	Timezone string `json:"timezone"`
}

type backendErrorResponse struct {
	Error backendError `json:"error"`
}

type backendError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
