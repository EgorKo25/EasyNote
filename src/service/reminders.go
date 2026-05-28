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

func NewReminderService(repository ReminderCommandRepository) *ReminderService {
	return &ReminderService{repository: repository}
}

type ReminderService struct {
	repository ReminderCommandRepository
}

//go:generate go run go.uber.org/mock/mockgen -source=reminders.go -destination=mocks/reminders_mock.go -package=mocks
type ReminderCommandRepository interface {
	CreateReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
	) (database.ReminderView, error)
	FindRemindersByChat(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error)
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

	reminder, err := s.repository.CreateReminder(ctx, telegramChatID, title, strings.TrimSpace(description), remindAt)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create reminder: %w", err)
	}

	return reminder, nil
}

func (s *ReminderService) Reminders(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error) {
	reminders, err := s.repository.FindRemindersByChat(ctx, telegramChatID)
	if err != nil {
		return nil, fmt.Errorf("find reminders: %w", err)
	}

	return reminders, nil
}

func (s *ReminderService) DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error {
	if err := s.repository.CancelReminder(ctx, telegramChatID, reminderID); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}

	return nil
}
