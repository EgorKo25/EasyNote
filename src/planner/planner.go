package planner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

var ErrUnsupportedRuleType = errors.New("unsupported reminder rule type")

//go:generate go run go.uber.org/mock/mockgen -source=planner.go -destination=mocks/planner_mock.go -package=mocks
type Storage interface {
	CreateReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
	) (database.ReminderView, error)
	CreateRecurringReminder(
		ctx context.Context,
		request database.RecurringReminderRequest,
	) (database.ReminderView, error)
	WithinTransaction(ctx context.Context, action func(ctx context.Context, storage TransactionStorage) error) error
}

type TransactionStorage interface {
	LoadOccurrenceForUpdate(ctx context.Context, reminderID int64) (database.Reminder, error)
	LoadHolidayForUpdate(ctx context.Context, holidayID int64) (database.Holiday, error)
	LoadRuleForUpdate(ctx context.Context, ruleID int64) (database.ReminderRule, error)
	LoadCustomDatesForUpdate(ctx context.Context, ruleID int64) ([]database.CustomReminderDate, error)
	MarkOccurrence(ctx context.Context, reminderID int64, status database.ReminderStatus, sentAt *time.Time) error
	MarkHolidayOccurrence(ctx context.Context, holiday database.Holiday, sentAt *time.Time) error
	CreateNextOccurrence(ctx context.Context, noteID int64, ruleID int64, remindAt time.Time) (database.Reminder, error)
	MarkCustomDateGenerated(ctx context.Context, ruleID int64, remindDate time.Time, reminderID int64) error
	UpdateRuleStatus(ctx context.Context, ruleID int64, status database.ReminderRuleStatus) error
}

func New(storage Storage) *Planner {
	return &Planner{storage: storage}
}

type Planner struct {
	storage Storage
}

func (p *Planner) CreateSingleOccurrence(
	ctx context.Context,
	telegramChatID int64,
	title string,
	description string,
	remindAt time.Time,
) (database.ReminderView, error) {
	reminder, err := p.storage.CreateReminder(ctx, telegramChatID, title, description, remindAt)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create single occurrence: %w", err)
	}

	return reminder, nil
}

func (p *Planner) CreateInitialOccurrenceForRule(
	ctx context.Context,
	request database.RecurringReminderRequest,
) (database.ReminderView, error) {
	reminder, err := p.storage.CreateRecurringReminder(ctx, request)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create initial occurrence for rule: %w", err)
	}

	return reminder, nil
}

func (p *Planner) CompleteOccurrence(
	ctx context.Context,
	reminderID int64,
	status database.ReminderStatus,
	sentAt *time.Time,
) error {
	if err := p.storage.WithinTransaction(ctx, func(ctx context.Context, storage TransactionStorage) error {
		occurrence, err := storage.LoadOccurrenceForUpdate(ctx, reminderID)
		if err != nil {
			return fmt.Errorf("load occurrence for update: %w", err)
		}

		if occurrence.Status != database.ReminderStatusNew {
			return nil
		}

		if err = storage.MarkOccurrence(ctx, reminderID, status, sentAt); err != nil {
			return fmt.Errorf("mark occurrence %s: %w", status, err)
		}

		if occurrence.RuleID == nil {
			return nil
		}

		return p.createNextOccurrence(ctx, storage, occurrence)
	}); err != nil {
		return fmt.Errorf("complete occurrence: %w", err)
	}

	return nil
}

func (p *Planner) CompleteHoliday(
	ctx context.Context,
	holidayID int64,
	sentAt *time.Time,
) error {
	if err := p.storage.WithinTransaction(ctx, func(ctx context.Context, storage TransactionStorage) error {
		holiday, err := storage.LoadHolidayForUpdate(ctx, holidayID)
		if err != nil {
			return fmt.Errorf("load holiday for update: %w", err)
		}

		if holiday.Status != database.HolidayStatusActive {
			return nil
		}

		if err = storage.MarkHolidayOccurrence(ctx, holiday, sentAt); err != nil {
			return fmt.Errorf("mark holiday occurrence: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("complete holiday: %w", err)
	}

	return nil
}

func (p *Planner) createNextOccurrence(
	ctx context.Context,
	storage TransactionStorage,
	occurrence database.Reminder,
) error {
	rule, customDates, err := loadRuleContext(ctx, storage, *occurrence.RuleID)
	if err != nil {
		return err
	}

	next, err := CalculateNextOccurrenceTime(occurrence.RemindAt, rule, customDates)
	if err != nil {
		return err
	}

	if next.CompleteRule {
		if err = storage.UpdateRuleStatus(ctx, rule.ID, database.ReminderRuleStatusCompleted); err != nil {
			return fmt.Errorf("complete rule: %w", err)
		}

		return nil
	}

	if next.RemindAt == nil {
		return nil
	}

	created, err := storage.CreateNextOccurrence(ctx, occurrence.NoteID, rule.ID, *next.RemindAt)
	if err != nil {
		return fmt.Errorf("create next occurrence: %w", err)
	}

	if rule.Type != database.ReminderRuleTypeCustomDates {
		return nil
	}

	if err = storage.MarkCustomDateGenerated(ctx, rule.ID, *next.RemindAt, created.ID); err != nil {
		return fmt.Errorf("mark custom date generated: %w", err)
	}

	return nil
}

func loadRuleContext(
	ctx context.Context,
	storage TransactionStorage,
	ruleID int64,
) (database.ReminderRule, []database.CustomReminderDate, error) {
	rule, err := storage.LoadRuleForUpdate(ctx, ruleID)
	if err != nil {
		return database.ReminderRule{}, nil, fmt.Errorf("load rule for update: %w", err)
	}

	if rule.Type != database.ReminderRuleTypeCustomDates {
		return rule, nil, nil
	}

	customDates, err := storage.LoadCustomDatesForUpdate(ctx, rule.ID)
	if err != nil {
		return database.ReminderRule{}, nil, fmt.Errorf("load custom dates for update: %w", err)
	}

	return rule, customDates, nil
}
