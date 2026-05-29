package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func NewReminderBot(sender *Sender, service ReminderService, cfg config.Telegram) *ReminderBot {
	return &ReminderBot{
		bot:         sender.bot,
		cfg:         cfg,
		service:     service,
		log:         logger.Get("telegram/reminder-bot"),
		draftByChat: make(map[int64]reminderDraft),
	}
}

type ReminderBot struct {
	bot     *telego.Bot
	cfg     config.Telegram
	service ReminderService
	log     logger.Logger

	mu          sync.Mutex
	draftByChat map[int64]reminderDraft
}

//go:generate go run go.uber.org/mock/mockgen -source=reminder_bot.go -destination=mocks/reminder_bot_mock.go -package=mocks
type ReminderService interface {
	CreateReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
	) (database.ReminderView, error)
	Reminders(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error)
	DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error
}

type reminderDraft struct {
	title       string
	description string
	remindAt    time.Time
	waitingFor  draftField
}

type draftField string

func (b *ReminderBot) Info() string {
	return "run telegram reminder bot"
}

func (b *ReminderBot) Run(ctx context.Context) error {
	updates, err := b.bot.UpdatesViaLongPolling(ctx, nil, b.longPollingOptions()...)
	if err != nil {
		return fmt.Errorf("start telegram long polling: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("run telegram reminder bot: %w", ctx.Err())
		case update, ok := <-updates:
			if !ok {
				return nil
			}

			if err = b.handleUpdate(ctx, update); err != nil {
				b.log.Error("cannot handle telegram update", err)
			}
		}
	}
}

func (b *ReminderBot) longPollingOptions() []telego.LongPollingOption {
	return []telego.LongPollingOption{
		telego.WithLongPollingBuffer(b.cfg.LongPollingBuffer),
		telego.WithLongPollingUpdateInterval(b.cfg.LongPollingInterval),
		telego.WithLongPollingRetryTimeout(b.cfg.LongPollingRetry),
	}
}

func (b *ReminderBot) handleUpdate(ctx context.Context, update telego.Update) error {
	if update.CallbackQuery != nil {
		return b.handleCallback(ctx, *update.CallbackQuery)
	}

	if update.Message != nil {
		return b.handleMessage(ctx, *update.Message)
	}

	return nil
}

func (b *ReminderBot) handleMessage(ctx context.Context, message telego.Message) error {
	chatID := message.Chat.ID
	text := strings.TrimSpace(message.Text)

	if text == "/start" || text == buttonStart {
		return b.sendStart(ctx, chatID)
	}

	if text == buttonCreateReminder {
		return b.startReminderDraft(ctx, chatID)
	}

	if text == buttonReminderList {
		return b.sendReminderList(ctx, chatID)
	}

	if b.applyDraftText(chatID, text) {
		return b.sendDraft(ctx, chatID)
	}

	return b.sendMessage(ctx, chatID, "Выберите действие на клавиатуре ниже.")
}

func (b *ReminderBot) handleCallback(ctx context.Context, query telego.CallbackQuery) error {
	if query.Message == nil {
		return nil
	}

	chatID := query.Message.GetChat().ID
	if err := b.bot.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID)); err != nil {
		return fmt.Errorf("answer callback: %w", err)
	}

	if query.Data == callbackDraftTitle {
		b.setDraftWaitingFor(chatID, fieldTitle)

		return b.sendMessage(ctx, chatID, "Введите название напоминания.")
	}

	if query.Data == callbackDraftDescription {
		b.setDraftWaitingFor(chatID, fieldDescription)

		return b.sendMessage(ctx, chatID, "Введите описание напоминания.")
	}

	if query.Data == callbackDraftTime {
		b.setDraftWaitingFor(chatID, fieldTime)

		return b.sendMessage(ctx, chatID, "Введите время в формате 2006-01-02 15:04.")
	}

	if query.Data == callbackDraftSave {
		return b.saveDraft(ctx, chatID)
	}

	if query.Data == callbackDraftCancel {
		b.deleteDraft(chatID)

		return b.sendMessage(ctx, chatID, "Создание напоминания отменено.")
	}

	if strings.HasPrefix(query.Data, callbackDeletePrefix) {
		return b.deleteReminder(ctx, chatID, query.Data)
	}

	return nil
}

func (b *ReminderBot) sendStart(ctx context.Context, chatID int64) error {
	text := "Здравствуйте. Я помогу вести напоминания.\n\n" +
		"Сейчас я умею создавать напоминания, показывать список активных напоминаний и удалять их."

	return b.sendMessageWithKeyboard(ctx, chatID, text)
}

func (b *ReminderBot) startReminderDraft(ctx context.Context, chatID int64) error {
	b.mu.Lock()
	b.draftByChat[chatID] = reminderDraft{}
	b.mu.Unlock()

	return b.sendDraft(ctx, chatID)
}

func (b *ReminderBot) sendDraft(ctx context.Context, chatID int64) error {
	draft := b.draft(chatID)

	params := tu.Message(tu.ID(chatID), draftText(draft)).
		WithReplyMarkup(draftKeyboard())
	if _, err := b.bot.SendMessage(ctx, params); err != nil {
		return fmt.Errorf("send reminder draft: %w", err)
	}

	return nil
}

func (b *ReminderBot) sendReminderList(ctx context.Context, chatID int64) error {
	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	if len(reminders) == 0 {
		return b.sendMessage(ctx, chatID, "Активных напоминаний пока нет.")
	}

	for _, reminder := range reminders {
		params := tu.Message(tu.ID(chatID), reminderViewText(reminder)).
			WithReplyMarkup(deleteKeyboard(reminder.ID))
		if _, err = b.bot.SendMessage(ctx, params); err != nil {
			return fmt.Errorf("send reminder list item: %w", err)
		}
	}

	return nil
}

func (b *ReminderBot) saveDraft(ctx context.Context, chatID int64) error {
	draft := b.draft(chatID)
	if draft.title == "" || draft.remindAt.IsZero() {
		return b.sendMessage(ctx, chatID, "Добавьте название и время перед сохранением.")
	}

	reminder, err := b.service.CreateReminder(ctx, chatID, draft.title, draft.description, draft.remindAt)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}

	b.deleteDraft(chatID)

	return b.sendMessage(ctx, chatID, "Напоминание создано.\n\n"+reminderViewText(reminder))
}

func (b *ReminderBot) deleteReminder(ctx context.Context, chatID int64, data string) error {
	reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, callbackDeletePrefix), 10, 64)
	if err != nil {
		return fmt.Errorf("parse reminder id: %w", err)
	}

	if err = b.service.DeleteReminder(ctx, chatID, reminderID); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}

	return b.sendMessage(ctx, chatID, "Напоминание удалено.")
}

func (b *ReminderBot) sendMessage(ctx context.Context, chatID int64, text string) error {
	_, err := b.bot.SendMessage(ctx, tu.Message(tu.ID(chatID), text))
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}

	return nil
}

func (b *ReminderBot) sendMessageWithKeyboard(ctx context.Context, chatID int64, text string) error {
	params := tu.Message(tu.ID(chatID), text).WithReplyMarkup(mainKeyboard())
	if _, err := b.bot.SendMessage(ctx, params); err != nil {
		return fmt.Errorf("send telegram message with keyboard: %w", err)
	}

	return nil
}

func (b *ReminderBot) applyDraftText(chatID int64, text string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists || draft.waitingFor == "" {
		return false
	}

	switch draft.waitingFor {
	case fieldTitle:
		draft.title = text
	case fieldDescription:
		draft.description = text
	case fieldTime:
		remindAt, err := parseReminderTime(text)
		if err != nil {
			draft.waitingFor = fieldTime
			b.draftByChat[chatID] = draft

			return true
		}

		draft.remindAt = remindAt
	}

	draft.waitingFor = ""
	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) setDraftWaitingFor(chatID int64, field draftField) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.waitingFor = field
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) draft(chatID int64) reminderDraft {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.draftByChat[chatID]
}

func (b *ReminderBot) deleteDraft(chatID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.draftByChat, chatID)
}

func parseReminderTime(value string) (time.Time, error) {
	remindAt, err := time.ParseInLocation(time.DateTime, value, time.Local)
	if err == nil {
		return remindAt, nil
	}

	remindAt, err = time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse reminder time: %w", err)
	}

	return remindAt, nil
}
