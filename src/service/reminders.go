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
var ErrWeekdaysMaskEmpty = errors.New("weekdays mask is empty")

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
	FindRemindersByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]database.ReminderView, error)
	FindReminderHistoryByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]database.ReminderView, error)
	CancelReminder(ctx context.Context, telegramChatID int64, reminderID int64) error
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
	firstDate := earliestDate(selectedDates)
	firstRemindAt := time.Date(firstDate.Year(), firstDate.Month(), firstDate.Day(), hour, minute, 0, 0, location)
	rule := database.ReminderRule{Type: database.ReminderRuleTypeCustomDates}

	return s.createRecurringReminder(ctx, telegramChatID, title, description, firstRemindAt, rule, selectedDates)
}

func (s *ReminderService) DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error {
	if err := s.repository.CancelReminder(ctx, telegramChatID, reminderID); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
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
