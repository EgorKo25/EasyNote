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
		uiByChat:    make(map[int64]int),
	}
}

type ReminderBot struct {
	bot     *telego.Bot
	cfg     config.Telegram
	service ReminderService
	log     logger.Logger

	mu          sync.Mutex
	draftByChat map[int64]reminderDraft
	uiByChat    map[int64]int
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
	title         string
	description   string
	notice        string
	date          time.Time
	hour          int
	minute        int
	screen        draftScreen
	calendarMonth time.Time
	waitingFor    draftField
}

type draftField string
type draftScreen string

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
		return b.editDraft(ctx, chatID, b.currentUIMessageID(chatID))
	}

	return b.replaceUI(ctx, chatID, "Выберите действие на клавиатуре ниже.", mainKeyboard())
}

func (b *ReminderBot) handleCallback(ctx context.Context, query telego.CallbackQuery) error {
	if query.Message == nil {
		return nil
	}

	chatID := query.Message.GetChat().ID
	if err := b.bot.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID)); err != nil {
		return fmt.Errorf("answer callback: %w", err)
	}

	if query.Data == callbackNoop {
		return nil
	}

	messageID := query.Message.GetMessageID()
	if query.Data == callbackMainMenu {
		return b.editUI(ctx, chatID, messageID, mainMenuText(), nil)
	}

	if handled, err := b.handleDraftCommandCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if b.applyDraftDateCallback(chatID, query.Data) {
		return b.editDraft(ctx, chatID, messageID)
	}

	if strings.HasPrefix(query.Data, callbackDeletePrefix) {
		return b.deleteReminder(ctx, chatID, messageID, query.Data)
	}

	return nil
}

func (b *ReminderBot) handleDraftCommandCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch data {
	case callbackDraftTitle:
		b.setDraftWaitingFor(chatID, fieldTitle)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftDescription:
		b.setDraftWaitingFor(chatID, fieldDescription)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftTimeText:
		b.setDraftWaitingFor(chatID, fieldTime)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftDate:
		b.setDraftScreen(chatID, draftScreenDate)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftCalendar:
		b.setDraftScreen(chatID, draftScreenCalendar)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftBack:
		b.setDraftScreen(chatID, draftScreenMain)

		return true, b.editDraft(ctx, chatID, messageID)
	default:
		return b.handleDraftFinalCommandCallback(ctx, chatID, messageID, data)
	}
}

func (b *ReminderBot) handleDraftFinalCommandCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch data {
	case callbackDraftSave:
		return true, b.saveDraft(ctx, chatID, messageID)
	case callbackDraftTimeDone:
		b.setDraftScreen(chatID, draftScreenMain)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftCancel:
		b.deleteDraft(chatID)

		return true, b.editUI(ctx, chatID, messageID, "Создание напоминания отменено.", nil)
	default:
		return false, nil
	}
}

func (b *ReminderBot) sendStart(ctx context.Context, chatID int64) error {
	return b.replaceUI(ctx, chatID, mainMenuText(), mainKeyboard())
}

func (b *ReminderBot) startReminderDraft(ctx context.Context, chatID int64) error {
	b.mu.Lock()
	b.draftByChat[chatID] = newReminderDraft()
	b.mu.Unlock()

	return b.replaceDraft(ctx, chatID)
}

func (b *ReminderBot) replaceDraft(ctx context.Context, chatID int64) error {
	draft := b.draft(chatID)

	return b.replaceUI(ctx, chatID, draftText(draft), draftKeyboard(draft))
}

func (b *ReminderBot) editDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)

	return b.editUI(ctx, chatID, messageID, draftText(draft), draftKeyboard(draft))
}

func (b *ReminderBot) sendReminderList(ctx context.Context, chatID int64) error {
	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	return b.replaceUI(ctx, chatID, reminderListText(reminders), reminderListKeyboard(reminders))
}

func (b *ReminderBot) saveDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)

	remindAt := draft.remindAt()
	if draft.title == "" || remindAt.IsZero() {
		b.setDraftNotice(chatID, "Чтобы сохранить напоминание, добавьте название и дату с временем.")

		return b.editDraft(ctx, chatID, messageID)
	}

	reminder, err := b.service.CreateReminder(ctx, chatID, draft.title, draft.description, remindAt)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}

	b.deleteDraft(chatID)

	return b.editUI(ctx, chatID, messageID, "Напоминание создано.\n\n"+reminderViewText(reminder), nil)
}

func (b *ReminderBot) deleteReminder(ctx context.Context, chatID int64, messageID int, data string) error {
	reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, callbackDeletePrefix), 10, 64)
	if err != nil {
		return fmt.Errorf("parse reminder id: %w", err)
	}

	if err = b.service.DeleteReminder(ctx, chatID, reminderID); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}

	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders after delete: %w", err)
	}

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"Напоминание удалено.\n\n"+reminderListText(reminders),
		reminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) replaceUI(ctx context.Context, chatID int64, text string, replyMarkup telego.ReplyMarkup) error {
	b.deleteCurrentUI(ctx, chatID)

	params := tu.Message(tu.ID(chatID), text)
	if replyMarkup != nil {
		params.WithReplyMarkup(replyMarkup)
	}

	message, err := b.bot.SendMessage(ctx, params)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}

	if message != nil {
		b.setCurrentUIMessageID(chatID, message.MessageID)
	}

	return nil
}

func (b *ReminderBot) editUI(
	ctx context.Context,
	chatID int64,
	messageID int,
	text string,
	replyMarkup *telego.InlineKeyboardMarkup,
) error {
	if messageID == 0 {
		messageID = b.currentUIMessageID(chatID)
	}

	if messageID == 0 {
		return b.replaceUI(ctx, chatID, text, replyMarkup)
	}

	params := tu.EditMessageText(tu.ID(chatID), messageID, text).WithReplyMarkup(replyMarkup)

	message, err := b.bot.EditMessageText(ctx, params)
	if err != nil {
		return fmt.Errorf("edit telegram message: %w", err)
	}

	if message != nil {
		b.setCurrentUIMessageID(chatID, message.MessageID)
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
		parsed, err := time.Parse("15:04", text)
		if err != nil {
			draft.notice = "Не понял время. Отправьте его в формате 09:30."
			draft.waitingFor = fieldTime
			draft.screen = draftScreenTime
			b.draftByChat[chatID] = draft

			return true
		}

		draft.hour = parsed.Hour()
		draft.minute = parsed.Minute()
	}

	draft.notice = ""
	draft.waitingFor = ""
	draft.screen = draftScreenMain
	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) applyDraftDateCallback(chatID int64, data string) bool {
	if value, ok := strings.CutPrefix(data, callbackMonthPrefix); ok {
		return b.applyDraftMonth(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, callbackDatePrefix); ok {
		return b.applyDraftDate(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, callbackTimeSetPrefix); ok {
		return b.applyDraftTime(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, callbackTimeShiftPrefix); ok {
		return b.applyDraftTimeShift(chatID, value)
	}

	return false
}

func (b *ReminderBot) applyDraftMonth(chatID int64, value string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	month, err := time.ParseInLocation("2006-01", value, time.Local)
	if err != nil {
		return false
	}

	draft.calendarMonth = firstDayOfMonth(month)
	draft.notice = ""
	draft.screen = draftScreenCalendar
	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) applyDraftDate(chatID int64, value string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	date, err := time.ParseInLocation(time.DateOnly, value, time.Local)
	if err != nil {
		return false
	}

	draft.date = date
	draft.calendarMonth = firstDayOfMonth(date)
	draft.notice = ""
	draft.screen = draftScreenTime
	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) applyDraftTime(chatID int64, value string) bool {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	draft.hour = parsed.Hour()
	draft.minute = parsed.Minute()
	draft.notice = ""
	draft.screen = draftScreenTime
	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) applyDraftTimeShift(chatID int64, value string) bool {
	minutes, err := strconv.Atoi(value)
	if err != nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	selectedTime := time.Date(2000, 1, 1, draft.hour, draft.minute, 0, 0, time.Local)
	shiftedTime := selectedTime.Add(time.Duration(minutes) * time.Minute)
	draft.hour = shiftedTime.Hour()
	draft.minute = shiftedTime.Minute()
	draft.notice = ""
	draft.screen = draftScreenTime
	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) setDraftWaitingFor(chatID int64, field draftField) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.waitingFor = field
	draft.notice = ""
	draft.screen = draftScreenMain
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftNotice(chatID int64, notice string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.notice = notice
	draft.waitingFor = ""
	draft.screen = draftScreenMain
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftScreen(chatID int64, screen draftScreen) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	if draft.calendarMonth.IsZero() {
		draft.calendarMonth = firstDayOfMonth(time.Now())
	}

	draft.waitingFor = ""
	draft.notice = ""
	draft.screen = screen
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

func (b *ReminderBot) setCurrentUIMessageID(chatID int64, messageID int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.uiByChat[chatID] = messageID
}

func (b *ReminderBot) currentUIMessageID(chatID int64) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.uiByChat[chatID]
}

func (b *ReminderBot) deleteCurrentUI(ctx context.Context, chatID int64) {
	messageID := b.currentUIMessageID(chatID)
	if messageID == 0 {
		return
	}

	b.deleteMessage(ctx, chatID, messageID)
}

func (b *ReminderBot) deleteMessage(ctx context.Context, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}

	err := b.bot.DeleteMessage(ctx, &telego.DeleteMessageParams{
		ChatID:    tu.ID(chatID),
		MessageID: messageID,
	})
	if err != nil {
		b.log.Warn("cannot delete telegram message", logger.Any("error", err))
	}
}

func newReminderDraft() reminderDraft {
	now := time.Now().Add(time.Hour)

	return reminderDraft{
		hour:          now.Hour(),
		minute:        now.Minute() / 5 * 5,
		screen:        draftScreenMain,
		calendarMonth: firstDayOfMonth(now),
	}
}

func (d reminderDraft) remindAt() time.Time {
	if d.date.IsZero() {
		return time.Time{}
	}

	return time.Date(d.date.Year(), d.date.Month(), d.date.Day(), d.hour, d.minute, 0, 0, time.Local)
}

func (d reminderDraft) selectedTimeText() string {
	return fmt.Sprintf("%02d:%02d", d.hour, d.minute)
}
