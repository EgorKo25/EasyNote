package commands

import (
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

type EmptyResponse struct{}

type HealthResponse struct {
	Status string `json:"status"`
}

type TelegramChatRequest struct {
	TelegramChatID int64
}

type PaginationRequest struct {
	Offset         int
	Limit          int
	TelegramChatID int64
}

type ReminderRequest struct {
	TelegramChatID int64
	ReminderID     int64
}

type AcknowledgeReminderRequest struct {
	TelegramChatID int64
	ReminderID     int64
}

type CreateReminderRequest struct {
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	RemindAt       time.Time       `json:"remind_at"`
	Repeat         *ReminderRepeat `json:"repeat"`
	TelegramChatID int64           `json:"-"`
}

type UpdateReminderRequest struct {
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	RemindAt       time.Time `json:"remind_at"`
	TelegramChatID int64     `json:"-"`
	ReminderID     int64     `json:"-"`
}

type ReminderRepeat struct {
	Type         string   `json:"type"`
	IntervalDays *int     `json:"interval_days,omitempty"`
	WeekdaysMask *int     `json:"weekdays_mask,omitempty"`
	Dates        []string `json:"dates"`
	Hour         *int     `json:"hour,omitempty"`
	Minute       *int     `json:"minute,omitempty"`
	Timezone     string   `json:"timezone,omitempty"`
}

type ReminderDTO struct {
	ID          int64                   `json:"id"`
	RuleID      *int64                  `json:"rule_id"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	RemindAt    time.Time               `json:"remind_at"`
	Status      database.ReminderStatus `json:"status"`
	CreatedAt   time.Time               `json:"created_at"`
}

type ReminderResponse struct {
	Reminder ReminderDTO `json:"reminder"`
}

type ReminderListResponse struct {
	Reminders []ReminderDTO `json:"reminders"`
}

type HolidayRequest struct {
	TelegramChatID int64
	HolidayID      int64
}

type CreateHolidayRequest struct {
	HolidayName      string    `json:"holiday_name"`
	RecipientName    string    `json:"recipient_name"`
	HolidayDate      string    `json:"holiday_date"`
	HolidayDateValue time.Time `json:"-"`
	RemindAt         time.Time `json:"remind_at"`
	TelegramChatID   int64     `json:"-"`
}

type UpdateHolidayRequest struct {
	HolidayName      string    `json:"holiday_name"`
	RecipientName    string    `json:"recipient_name"`
	HolidayDate      string    `json:"holiday_date"`
	HolidayDateValue time.Time `json:"-"`
	RemindAt         time.Time `json:"remind_at"`
	TelegramChatID   int64     `json:"-"`
	HolidayID        int64     `json:"-"`
}

type HolidayDTO struct {
	ID             int64                  `json:"id"`
	TelegramChatID int64                  `json:"telegram_chat_id"`
	HolidayName    string                 `json:"holiday_name"`
	RecipientName  string                 `json:"recipient_name"`
	HolidayDate    time.Time              `json:"holiday_date"`
	RemindAt       time.Time              `json:"remind_at"`
	Status         database.HolidayStatus `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
}

type HolidayResponse struct {
	Holiday HolidayDTO `json:"holiday"`
}

type HolidayListResponse struct {
	Holidays []HolidayDTO `json:"holidays"`
}

type CreateShoppingListRequest struct {
	Items          []CreateShoppingListItemRequest `json:"items"`
	TelegramChatID int64                           `json:"-"`
}

type CreateShoppingListItemRequest struct {
	Text string `json:"text"`
}

type ShoppingListRequest struct {
	TelegramChatID int64
	ListID         int64
}

type ToggleShoppingListItemRequest struct {
	TelegramChatID int64
	ItemID         int64
}

type ShoppingListDTO struct {
	ID             int64                 `json:"id"`
	TelegramChatID int64                 `json:"telegram_chat_id"`
	CreatedAt      time.Time             `json:"created_at"`
	Items          []ShoppingListItemDTO `json:"items"`
}

type ShoppingListItemDTO struct {
	ID        int64     `json:"id"`
	ListID    int64     `json:"list_id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

type ShoppingListResponse struct {
	ShoppingList ShoppingListDTO `json:"shopping_list"`
}

type ShoppingListsResponse struct {
	ShoppingLists []ShoppingListDTO `json:"shopping_lists"`
}

type DashboardResponse struct {
	Stats ReminderStatsDTO `json:"stats"`
}

type ReminderStatsDTO struct {
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
	Skipped   int `json:"skipped"`
}

type UpdateSettingsRequest struct {
	TelegramChatID           int64   `json:"-"`
	Timezone                 *string `json:"timezone"`
	IntrusiveEnabled         *bool   `json:"intrusive_enabled"`
	IntrusiveIntervalMinutes *int    `json:"intrusive_interval_minutes"`
}

type SettingsResponse struct {
	Settings UserSettingsDTO `json:"settings"`
}

type UserSettingsDTO struct {
	TelegramChatID           int64     `json:"telegram_chat_id"`
	Language                 string    `json:"language"`
	Timezone                 string    `json:"timezone"`
	IntrusiveEnabled         bool      `json:"intrusive_enabled"`
	IntrusiveIntervalMinutes int       `json:"intrusive_interval_minutes"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type TimezoneListResponse struct {
	Timezones []TimezoneDTO `json:"timezones"`
}

type TimezoneDTO struct {
	City     string `json:"city"`
	Timezone string `json:"timezone"`
}
