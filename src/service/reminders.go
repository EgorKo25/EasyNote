package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

var ErrReminderTitleEmpty = errors.New("reminder title is empty")
var ErrCustomReminderDatesEmpty = errors.New("custom reminder dates are empty")
var ErrWeekdaysMaskEmpty = errors.New("weekdays mask is empty")
var ErrIntrusiveIntervalOutOfRange = errors.New("intrusive reminder interval is out of range")
var ErrUnknownTimezoneCity = errors.New("unknown timezone city")

func NewReminderService(repository ReminderCommandRepository, planner ReminderCreationPlanner) *ReminderService {
	return &ReminderService{repository: repository, ReminderCreationPlanner: planner}
}

type ReminderService struct {
	ReminderCreationPlanner

	repository ReminderCommandRepository
}

//go:generate go run go.uber.org/mock/mockgen -source=reminders.go -destination=mocks/reminders_mock.go -package=mocks
type ReminderCreationPlanner interface {
	CreateSingleOccurrence(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
	) (database.ReminderView, error)
	CreateInitialOccurrenceForRule(
		ctx context.Context,
		request database.RecurringReminderRequest,
	) (database.ReminderView, error)
	CompleteOccurrence(
		ctx context.Context,
		reminderID int64,
		status database.ReminderStatus,
		sentAt *time.Time,
	) error
}

type ReminderCommandRepository interface {
	FindRemindersByChat(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error)
	FindReminderHistoryByChat(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error)
	CancelReminder(ctx context.Context, telegramChatID int64, reminderID int64) error
	UserSettings(ctx context.Context, telegramChatID int64) (database.UserSettings, error)
	UpdateUserTimezone(ctx context.Context, telegramChatID int64, timezone string) error
	UpdateIntrusiveSettings(
		ctx context.Context,
		telegramChatID int64,
		enabled bool,
		interval time.Duration,
	) error
}

func (s *ReminderService) CreateReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
) (database.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return database.ReminderView{}, fmt.Errorf("create reminder: %w", ErrReminderTitleEmpty)
	}

	reminder, err := s.CreateSingleOccurrence(
		ctx,
		telegramChatID,
		title,
		strings.TrimSpace(description),
		remindAt,
	)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create reminder: %w", err)
	}

	return reminder, nil
}

func (s *ReminderService) CreateIntervalReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
	intervalDays int,
) (database.ReminderView, error) {
	rule := database.ReminderRule{
		Type:         database.ReminderRuleTypeInterval,
		IntervalDays: &intervalDays,
	}

	return s.createRecurringReminder(ctx, telegramChatID, title, description, remindAt, rule, nil)
}

func (s *ReminderService) CreateWeekdaysReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
	weekdaysMask int,
) (database.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return database.ReminderView{}, fmt.Errorf("create weekdays reminder: %w", ErrReminderTitleEmpty)
	}

	if weekdaysMask <= 0 {
		return database.ReminderView{}, fmt.Errorf("create weekdays reminder: %w", ErrWeekdaysMaskEmpty)
	}

	rule := database.ReminderRule{
		Type:         database.ReminderRuleTypeWeekdays,
		WeekdaysMask: &weekdaysMask,
	}

	return s.createRecurringReminder(ctx, telegramChatID, title, description, remindAt, rule, nil)
}

func (s *ReminderService) CreateCustomDatesReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	selectedDates []time.Time,
	hour int,
	minute int,
	location *time.Location,
) (database.ReminderView, error) {
	if len(selectedDates) == 0 {
		return database.ReminderView{}, fmt.Errorf("create custom dates reminder: %w", ErrCustomReminderDatesEmpty)
	}

	firstDate := earliestDate(selectedDates)
	firstRemindAt := time.Date(firstDate.Year(), firstDate.Month(), firstDate.Day(), hour, minute, 0, 0, location)
	rule := database.ReminderRule{Type: database.ReminderRuleTypeCustomDates}

	return s.createRecurringReminder(ctx, telegramChatID, title, description, firstRemindAt, rule, selectedDates)
}

func (s *ReminderService) Reminders(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error) {
	reminders, err := s.repository.FindRemindersByChat(ctx, telegramChatID)
	if err != nil {
		return nil, fmt.Errorf("find reminders: %w", err)
	}

	return reminders, nil
}

func (s *ReminderService) ReminderHistory(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error) {
	reminders, err := s.repository.FindReminderHistoryByChat(ctx, telegramChatID)
	if err != nil {
		return nil, fmt.Errorf("find reminder history: %w", err)
	}

	return reminders, nil
}

func (s *ReminderService) DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error {
	if err := s.repository.CancelReminder(ctx, telegramChatID, reminderID); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}

	return nil
}

func (s *ReminderService) CompleteReminder(ctx context.Context, reminderID int64) error {
	now := time.Now()
	if err := s.CompleteOccurrence(ctx, reminderID, database.ReminderStatusCompleted, &now); err != nil {
		return fmt.Errorf("complete reminder: %w", err)
	}

	return nil
}

func (s *ReminderService) CancelDeliveredReminder(ctx context.Context, reminderID int64) error {
	now := time.Now()
	if err := s.CompleteOccurrence(ctx, reminderID, database.ReminderStatusCanceled, &now); err != nil {
		return fmt.Errorf("cancel delivered reminder: %w", err)
	}

	return nil
}

func (s *ReminderService) UserSettings(ctx context.Context, telegramChatID int64) (database.UserSettings, error) {
	settings, err := s.repository.UserSettings(ctx, telegramChatID)
	if err != nil {
		return database.UserSettings{}, fmt.Errorf("load user settings: %w", err)
	}

	return settings, nil
}

func (s *ReminderService) SetUserTimezoneByCity(ctx context.Context, telegramChatID int64, city string) error {
	timezone, ok := timezoneForCity(city)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownTimezoneCity, city)
	}

	if err := s.repository.UpdateUserTimezone(ctx, telegramChatID, timezone); err != nil {
		return fmt.Errorf("set user timezone: %w", err)
	}

	return nil
}

func (s *ReminderService) SetIntrusiveSettings(
	ctx context.Context,
	telegramChatID int64,
	enabled bool,
	interval time.Duration,
) error {
	if interval < 30*time.Minute || interval > 24*time.Hour {
		return fmt.Errorf("set intrusive settings: %w", ErrIntrusiveIntervalOutOfRange)
	}

	if err := s.repository.UpdateIntrusiveSettings(ctx, telegramChatID, enabled, interval); err != nil {
		return fmt.Errorf("set intrusive settings: %w", err)
	}

	return nil
}

func (s *ReminderService) createRecurringReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
	rule database.ReminderRule,
	customDates []time.Time,
) (database.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return database.ReminderView{}, fmt.Errorf("create recurring reminder: %w", ErrReminderTitleEmpty)
	}

	reminder, err := s.CreateInitialOccurrenceForRule(ctx, database.RecurringReminderRequest{
		TelegramChatID: telegramChatID,
		Title:          title,
		Description:    strings.TrimSpace(description),
		FirstRemindAt:  remindAt,
		Rule:           rule,
		CustomDates:    customDates,
	})
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create recurring reminder: %w", err)
	}

	return reminder, nil
}

func earliestDate(dates []time.Time) time.Time {
	earliest := dates[0]
	for _, date := range dates[1:] {
		if date.Before(earliest) {
			earliest = date
		}
	}

	return earliest
}

func timezoneForCity(city string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(city))
	timezones := map[string]string{
		"москва":           "Europe/Moscow",
		"moscow":           "Europe/Moscow",
		"санкт-петербург":  "Europe/Moscow",
		"saint petersburg": "Europe/Moscow",
		"екатеринбург":     "Asia/Yekaterinburg",
		"новосибирск":      "Asia/Novosibirsk",
		"красноярск":       "Asia/Krasnoyarsk",
		"иркутск":          "Asia/Irkutsk",
		"владивосток":      "Asia/Vladivostok",
		"new york":         "America/New_York",
		"los angeles":      "America/Los_Angeles",
		"chicago":          "America/Chicago",
		"denver":           "America/Denver",
		"london":           "Europe/London",
		"berlin":           "Europe/Berlin",
		"paris":            "Europe/Paris",
		"tokyo":            "Asia/Tokyo",
	}

	timezone, ok := timezones[normalized]

	return timezone, ok
}
