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
	ReminderByID(ctx context.Context, telegramChatID int64, reminderID int64) (database.ReminderView, error)
	UpdateReminder(
		ctx context.Context,
		telegramChatID int64,
		reminderID int64,
		title string,
		description string,
		remindAt time.Time,
	) (database.ReminderView, error)
	CancelReminder(ctx context.Context, telegramChatID int64, reminderID int64) error
	CreateHoliday(ctx context.Context, request database.HolidayRequest) (database.HolidayView, error)
	UpdateHoliday(ctx context.Context, request database.HolidayUpdateRequest) (database.HolidayView, error)
	FindHolidaysByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]database.HolidayView, error)
	HolidayByID(ctx context.Context, telegramChatID int64, holidayID int64) (database.HolidayView, error)
	DeleteHoliday(ctx context.Context, telegramChatID int64, holidayID int64) error
	CreateShoppingList(
		ctx context.Context,
		telegramChatID int64,
		items []database.ShoppingListItem,
	) (database.ShoppingListView, error)
	FindShoppingListsByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]database.ShoppingListView, error)
	ShoppingListByID(ctx context.Context, telegramChatID int64, listID int64) (database.ShoppingListView, error)
	ToggleShoppingListItem(ctx context.Context, telegramChatID int64, itemID int64) (database.ShoppingListView, error)
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

func (s *ReminderService) ReminderByID(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
) (database.ReminderView, error) {
	reminder, err := s.repository.ReminderByID(ctx, telegramChatID, reminderID)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("load reminder by id: %w", err)
	}

	return reminder, nil
}

func (s *ReminderService) UpdateReminder(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
	title string,
	description string,
	remindAt time.Time,
) (database.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return database.ReminderView{}, fmt.Errorf("update reminder: %w", ErrReminderTitleEmpty)
	}

	reminder, err := s.repository.UpdateReminder(
		ctx,
		telegramChatID,
		reminderID,
		title,
		strings.TrimSpace(description),
		remindAt,
	)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("update reminder: %w", err)
	}

	return reminder, nil
}

func (s *ReminderService) CreateHoliday(
	ctx context.Context,
	request database.HolidayRequest,
) (database.HolidayView, error) {
	request.HolidayName = strings.TrimSpace(request.HolidayName)
	request.RecipientName = strings.TrimSpace(request.RecipientName)

	holiday, err := s.repository.CreateHoliday(ctx, request)
	if err != nil {
		return database.HolidayView{}, fmt.Errorf("create holiday: %w", err)
	}

	return holiday, nil
}

func (s *ReminderService) UpdateHoliday(
	ctx context.Context,
	request database.HolidayUpdateRequest,
) (database.HolidayView, error) {
	request.HolidayName = strings.TrimSpace(request.HolidayName)
	request.RecipientName = strings.TrimSpace(request.RecipientName)

	holiday, err := s.repository.UpdateHoliday(ctx, request)
	if err != nil {
		return database.HolidayView{}, fmt.Errorf("update holiday: %w", err)
	}

	return holiday, nil
}

func (s *ReminderService) FindHolidaysByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]database.HolidayView, error) {
	holidays, err := s.repository.FindHolidaysByChat(ctx, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find holidays by chat: %w", err)
	}

	return holidays, nil
}

func (s *ReminderService) HolidayByID(
	ctx context.Context,
	telegramChatID int64,
	holidayID int64,
) (database.HolidayView, error) {
	holiday, err := s.repository.HolidayByID(ctx, telegramChatID, holidayID)
	if err != nil {
		return database.HolidayView{}, fmt.Errorf("load holiday by id: %w", err)
	}

	return holiday, nil
}

func (s *ReminderService) DeleteHoliday(ctx context.Context, telegramChatID int64, holidayID int64) error {
	if err := s.repository.DeleteHoliday(ctx, telegramChatID, holidayID); err != nil {
		return fmt.Errorf("delete holiday: %w", err)
	}

	return nil
}

func (s *ReminderService) CreateShoppingList(
	ctx context.Context,
	telegramChatID int64,
	items []database.ShoppingListItem,
) (database.ShoppingListView, error) {
	list, err := s.repository.CreateShoppingList(ctx, telegramChatID, items)
	if err != nil {
		return database.ShoppingListView{}, fmt.Errorf("create shopping list: %w", err)
	}

	return list, nil
}

func (s *ReminderService) FindShoppingListsByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]database.ShoppingListView, error) {
	lists, err := s.repository.FindShoppingListsByChat(ctx, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find shopping lists by chat: %w", err)
	}

	return lists, nil
}

func (s *ReminderService) ShoppingListByID(
	ctx context.Context,
	telegramChatID int64,
	listID int64,
) (database.ShoppingListView, error) {
	list, err := s.repository.ShoppingListByID(ctx, telegramChatID, listID)
	if err != nil {
		return database.ShoppingListView{}, fmt.Errorf("load shopping list by id: %w", err)
	}

	return list, nil
}

func (s *ReminderService) ToggleShoppingListItem(
	ctx context.Context,
	telegramChatID int64,
	itemID int64,
) (database.ShoppingListView, error) {
	list, err := s.repository.ToggleShoppingListItem(ctx, telegramChatID, itemID)
	if err != nil {
		return database.ShoppingListView{}, fmt.Errorf("toggle shopping list item: %w", err)
	}

	return list, nil
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
