package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/domain"
)

func NewReminderService(repository ReminderCommandRepository, planner ReminderCreationPlanner) *ReminderService {
	return &ReminderService{repository: repository, planner: planner}
}

type ReminderService struct {
	repository ReminderCommandRepository
	planner    ReminderCreationPlanner
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
		statuses []database.ReminderStatus,
		limit int,
		offset int,
	) ([]database.ReminderView, error)
	ReminderStatsByChat(ctx context.Context, telegramChatID int64) (database.ReminderStats, error)
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
	AcknowledgeReminder(
		ctx context.Context,
		telegramChatID int64,
		reminderID int64,
		status database.ReminderStatus,
	) error
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
) (domain.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.ReminderView{}, fmt.Errorf("create reminder: %w", ErrReminderTitleEmpty)
	}

	reminder, err := s.planner.CreateSingleOccurrence(
		ctx,
		telegramChatID,
		title,
		strings.TrimSpace(description),
		remindAt,
	)
	if err != nil {
		return domain.ReminderView{}, fmt.Errorf("create reminder: %w", err)
	}

	return mapReminderViewToDomain(reminder), nil
}

func (s *ReminderService) CreateIntervalReminder(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
	intervalDays int,
) (domain.ReminderView, error) {
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
) (domain.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.ReminderView{}, fmt.Errorf("create weekdays reminder: %w", ErrReminderTitleEmpty)
	}

	if weekdaysMask <= 0 {
		return domain.ReminderView{}, fmt.Errorf("create weekdays reminder: %w", ErrWeekdaysMaskEmpty)
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
) (domain.ReminderView, error) {
	firstDate := earliestDate(selectedDates)
	firstRemindAt := time.Date(firstDate.Year(), firstDate.Month(), firstDate.Day(), hour, minute, 0, 0, location)
	rule := database.ReminderRule{Type: database.ReminderRuleTypeCustomDates}

	return s.createRecurringReminder(ctx, telegramChatID, title, description, firstRemindAt, rule, selectedDates)
}

func (s *ReminderService) DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error {
	if err := s.repository.CancelReminder(ctx, telegramChatID, reminderID); err != nil {
		if errors.Is(err, database.ErrReminderNotFound) {
			return fmt.Errorf("delete reminder: %w", ErrReminderNotFound)
		}

		return fmt.Errorf("delete reminder: %w", err)
	}

	return nil
}

func (s *ReminderService) AcknowledgeReminder(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
	status domain.ReminderStatus,
) error {
	if status != domain.ReminderStatusCompleted && status != domain.ReminderStatusCanceled {
		return fmt.Errorf("acknowledge reminder: %w", ErrReminderStatusUnsupported)
	}

	if err := s.repository.AcknowledgeReminder(
		ctx,
		telegramChatID,
		reminderID,
		database.ReminderStatus(status),
	); err != nil {
		if errors.Is(err, database.ErrReminderNotFound) {
			return fmt.Errorf("acknowledge reminder: %w", ErrReminderNotFound)
		}

		return fmt.Errorf("acknowledge reminder: %w", err)
	}

	return nil
}

func (s *ReminderService) ReminderByID(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
) (domain.ReminderView, error) {
	reminder, err := s.repository.ReminderByID(ctx, telegramChatID, reminderID)
	if err != nil {
		if errors.Is(err, database.ErrReminderNotFound) {
			return domain.ReminderView{}, fmt.Errorf("load reminder by id: %w", ErrReminderNotFound)
		}

		return domain.ReminderView{}, fmt.Errorf("load reminder by id: %w", err)
	}

	return mapReminderViewToDomain(reminder), nil
}

func (s *ReminderService) UpdateReminder(
	ctx context.Context,
	telegramChatID int64,
	reminderID int64,
	title string,
	description string,
	remindAt time.Time,
) (domain.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.ReminderView{}, fmt.Errorf("update reminder: %w", ErrReminderTitleEmpty)
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
		if errors.Is(err, database.ErrReminderNotFound) {
			return domain.ReminderView{}, fmt.Errorf("update reminder: %w", ErrReminderNotFound)
		}

		return domain.ReminderView{}, fmt.Errorf("update reminder: %w", err)
	}

	return mapReminderViewToDomain(reminder), nil
}

func (s *ReminderService) CreateHoliday(
	ctx context.Context,
	request domain.HolidayRequest,
) (domain.HolidayView, error) {
	request.HolidayName = strings.TrimSpace(request.HolidayName)
	request.RecipientName = strings.TrimSpace(request.RecipientName)

	holiday, err := s.repository.CreateHoliday(ctx, mapHolidayRequestToDatabase(request))
	if err != nil {
		return domain.HolidayView{}, fmt.Errorf("create holiday: %w", err)
	}

	return mapHolidayViewToDomain(holiday), nil
}

func (s *ReminderService) UpdateHoliday(
	ctx context.Context,
	request domain.HolidayUpdateRequest,
) (domain.HolidayView, error) {
	request.HolidayName = strings.TrimSpace(request.HolidayName)
	request.RecipientName = strings.TrimSpace(request.RecipientName)

	holiday, err := s.repository.UpdateHoliday(ctx, mapHolidayUpdateRequestToDatabase(request))
	if err != nil {
		if errors.Is(err, database.ErrHolidayNotFound) {
			return domain.HolidayView{}, fmt.Errorf("update holiday: %w", ErrHolidayNotFound)
		}

		return domain.HolidayView{}, fmt.Errorf("update holiday: %w", err)
	}

	return mapHolidayViewToDomain(holiday), nil
}

func (s *ReminderService) FindRemindersByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]domain.ReminderView, error) {
	reminders, err := s.repository.FindRemindersByChat(
		ctx,
		telegramChatID,
		[]database.ReminderStatus{database.ReminderStatusNew},
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("find reminders: %w", err)
	}

	return mapReminderViewsToDomain(reminders), nil
}

func (s *ReminderService) FindReminderHistoryByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]domain.ReminderView, error) {
	statuses := []database.ReminderStatus{
		database.ReminderStatusSent,
		database.ReminderStatusFailed,
		database.ReminderStatusCanceled,
		database.ReminderStatusCompleted,
		database.ReminderStatusSkipped,
	}

	reminders, err := s.repository.FindRemindersByChat(ctx, telegramChatID, statuses, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find reminder history: %w", err)
	}

	return mapReminderViewsToDomain(reminders), nil
}

func (s *ReminderService) FindRemindersByStatuses(
	ctx context.Context,
	telegramChatID int64,
	statuses []domain.ReminderStatus,
	limit int,
	offset int,
) ([]domain.ReminderView, error) {
	databaseStatuses := make([]database.ReminderStatus, 0, len(statuses))
	for _, status := range statuses {
		databaseStatuses = append(databaseStatuses, database.ReminderStatus(status))
	}

	reminders, err := s.repository.FindRemindersByChat(ctx, telegramChatID, databaseStatuses, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find reminders by statuses: %w", err)
	}

	return mapReminderViewsToDomain(reminders), nil
}

func (s *ReminderService) ReminderStatsByChat(
	ctx context.Context,
	telegramChatID int64,
) (domain.ReminderStats, error) {
	stats, err := s.repository.ReminderStatsByChat(ctx, telegramChatID)
	if err != nil {
		return domain.ReminderStats{}, fmt.Errorf("load reminder stats by chat: %w", err)
	}

	return mapReminderStatsToDomain(stats), nil
}

func (s *ReminderService) FindHolidaysByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]domain.HolidayView, error) {
	holidays, err := s.repository.FindHolidaysByChat(ctx, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find holidays by chat: %w", err)
	}

	return mapHolidayViewsToDomain(holidays), nil
}

func (s *ReminderService) HolidayByID(
	ctx context.Context,
	telegramChatID int64,
	holidayID int64,
) (domain.HolidayView, error) {
	holiday, err := s.repository.HolidayByID(ctx, telegramChatID, holidayID)
	if err != nil {
		if errors.Is(err, database.ErrHolidayNotFound) {
			return domain.HolidayView{}, fmt.Errorf("load holiday by id: %w", ErrHolidayNotFound)
		}

		return domain.HolidayView{}, fmt.Errorf("load holiday by id: %w", err)
	}

	return mapHolidayViewToDomain(holiday), nil
}

func (s *ReminderService) DeleteHoliday(ctx context.Context, telegramChatID int64, holidayID int64) error {
	if err := s.repository.DeleteHoliday(ctx, telegramChatID, holidayID); err != nil {
		if errors.Is(err, database.ErrHolidayNotFound) {
			return fmt.Errorf("delete holiday: %w", ErrHolidayNotFound)
		}

		return fmt.Errorf("delete holiday: %w", err)
	}

	return nil
}

func (s *ReminderService) CreateShoppingList(
	ctx context.Context,
	telegramChatID int64,
	items []domain.ShoppingListItem,
) (domain.ShoppingListView, error) {
	list, err := s.repository.CreateShoppingList(ctx, telegramChatID, mapShoppingListItemsToDatabase(items))
	if err != nil {
		return domain.ShoppingListView{}, fmt.Errorf("create shopping list: %w", err)
	}

	return mapShoppingListViewToDomain(list), nil
}

func (s *ReminderService) FindShoppingListsByChat(
	ctx context.Context,
	telegramChatID int64,
	limit int,
	offset int,
) ([]domain.ShoppingListView, error) {
	lists, err := s.repository.FindShoppingListsByChat(ctx, telegramChatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find shopping lists by chat: %w", err)
	}

	return mapShoppingListViewsToDomain(lists), nil
}

func (s *ReminderService) ShoppingListByID(
	ctx context.Context,
	telegramChatID int64,
	listID int64,
) (domain.ShoppingListView, error) {
	list, err := s.repository.ShoppingListByID(ctx, telegramChatID, listID)
	if err != nil {
		if errors.Is(err, database.ErrShoppingListNotFound) {
			return domain.ShoppingListView{}, fmt.Errorf("load shopping list by id: %w", ErrShoppingListNotFound)
		}

		return domain.ShoppingListView{}, fmt.Errorf("load shopping list by id: %w", err)
	}

	return mapShoppingListViewToDomain(list), nil
}

func (s *ReminderService) ToggleShoppingListItem(
	ctx context.Context,
	telegramChatID int64,
	itemID int64,
) (domain.ShoppingListView, error) {
	list, err := s.repository.ToggleShoppingListItem(ctx, telegramChatID, itemID)
	if err != nil {
		if errors.Is(err, database.ErrShoppingListNotFound) {
			return domain.ShoppingListView{}, fmt.Errorf("toggle shopping list item: %w", ErrShoppingListNotFound)
		}

		return domain.ShoppingListView{}, fmt.Errorf("toggle shopping list item: %w", err)
	}

	return mapShoppingListViewToDomain(list), nil
}

func (s *ReminderService) UserSettings(
	ctx context.Context,
	telegramChatID int64,
) (domain.UserSettings, error) {
	settings, err := s.repository.UserSettings(ctx, telegramChatID)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("load user settings: %w", err)
	}

	return mapUserSettingsToDomain(settings), nil
}

func (s *ReminderService) UpdateUserTimezone(ctx context.Context, telegramChatID int64, timezone string) error {
	if err := s.repository.UpdateUserTimezone(ctx, telegramChatID, timezone); err != nil {
		return fmt.Errorf("update user timezone: %w", err)
	}

	return nil
}

func (s *ReminderService) UpdateIntrusiveSettings(
	ctx context.Context,
	telegramChatID int64,
	enabled bool,
	interval time.Duration,
) error {
	if err := s.repository.UpdateIntrusiveSettings(ctx, telegramChatID, enabled, interval); err != nil {
		return fmt.Errorf("update intrusive settings: %w", err)
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
) (domain.ReminderView, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.ReminderView{}, fmt.Errorf("create recurring reminder: %w", ErrReminderTitleEmpty)
	}

	reminder, err := s.planner.CreateInitialOccurrenceForRule(ctx, database.RecurringReminderRequest{
		TelegramChatID: telegramChatID,
		Title:          title,
		Description:    strings.TrimSpace(description),
		FirstRemindAt:  remindAt,
		Rule:           rule,
		CustomDates:    customDates,
	})
	if err != nil {
		return domain.ReminderView{}, fmt.Errorf("create recurring reminder: %w", err)
	}

	return mapReminderViewToDomain(reminder), nil
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
