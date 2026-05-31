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
		if err := b.editDraft(ctx, chatID, b.currentUIMessageID(chatID)); err != nil {
			return err
		}

		b.deleteMessage(ctx, chatID, message.MessageID)

		return nil
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
		return b.replaceUI(ctx, chatID, mainMenuText(), mainKeyboard())
	}

	if handled, err := b.handleDraftCommandCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if b.applyDraftDateCallback(chatID, query.Data) {
		return b.editDraft(ctx, chatID, messageID)
	}

	return b.handleReminderCallback(ctx, chatID, messageID, query.Data)
}

func (b *ReminderBot) handleReminderCallback(ctx context.Context, chatID int64, messageID int, data string) error {
	switch {
	case data == callbackCreateReminder:
		return b.startReminderDraftInMessage(ctx, chatID, messageID)
	case data == callbackReminderList || data == callbackReminderBackList:
		return b.editReminderList(ctx, chatID, messageID)
	case strings.HasPrefix(data, callbackDeletePrefix):
		return b.deleteReminder(ctx, chatID, messageID, data)
	case strings.HasPrefix(data, callbackViewPrefix):
		return b.viewReminder(ctx, chatID, messageID, data)
	default:
		return nil
	}
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

		return true, b.replaceUI(ctx, chatID, "Создание напоминания отменено.\n\n"+mainMenuText(), mainKeyboard())
	default:
		return false, nil
	}
}

func (b *ReminderBot) sendStart(ctx context.Context, chatID int64) error {
	return b.replaceUI(ctx, chatID, mainMenuText(), mainKeyboard())
}

func (b *ReminderBot) startReminderDraft(ctx context.Context, chatID int64) error {
	b.mu.Lock()
	b.draftByChat[chatID] = newReminderDraft(b.now())
	b.mu.Unlock()

	return b.replaceDraft(ctx, chatID)
}

func (b *ReminderBot) startReminderDraftInMessage(ctx context.Context, chatID int64, messageID int) error {
	b.mu.Lock()
	b.draftByChat[chatID] = newReminderDraft(b.now())
	b.mu.Unlock()

	return b.editDraft(ctx, chatID, messageID)
}

func (b *ReminderBot) replaceDraft(ctx context.Context, chatID int64) error {
	draft := b.draft(chatID)

	return b.replaceUI(ctx, chatID, draftText(draft, b.location()), draftKeyboard(draft, b.now()))
}

func (b *ReminderBot) editDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)

	return b.editUI(ctx, chatID, messageID, draftText(draft, b.location()), draftKeyboard(draft, b.now()))
}

func (b *ReminderBot) sendReminderList(ctx context.Context, chatID int64) error {
	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	return b.replaceUI(ctx, chatID, reminderListText(reminders, b.location()), reminderListKeyboard(reminders))
}

func (b *ReminderBot) editReminderList(ctx context.Context, chatID int64, messageID int) error {
	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	return b.editUI(ctx, chatID, messageID, reminderListText(reminders, b.location()), reminderListKeyboard(reminders))
}

func (b *ReminderBot) saveDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)

	remindAt := draft.remindAt(b.location())
	if draft.title == "" || remindAt.IsZero() {
		b.setDraftNotice(chatID, "Чтобы сохранить напоминание, добавьте название и дату с временем.")

		return b.editDraft(ctx, chatID, messageID)
	}

	if !remindAt.After(b.now()) {
		b.setDraftTimeNotice(chatID, "Это время уже прошло. Выберите будущую дату и время.")

		return b.editDraft(ctx, chatID, messageID)
	}

	reminder, err := b.service.CreateReminder(ctx, chatID, draft.title, draft.description, remindAt)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}

	b.deleteDraft(chatID)

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"Напоминание создано.\n\n"+reminderViewText(reminder, b.location()),
		reminderCreatedKeyboard(),
	)
}

func (b *ReminderBot) viewReminder(ctx context.Context, chatID int64, messageID int, data string) error {
	reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, callbackViewPrefix), 10, 64)
	if err != nil {
		return fmt.Errorf("parse reminder id: %w", err)
	}

	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	for _, reminder := range reminders {
		if reminder.ID == reminderID {
			return b.editUI(
				ctx,
				chatID,
				messageID,
				reminderViewText(reminder, b.location()),
				reminderDetailsKeyboard(reminder),
			)
		}
	}

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"Напоминание не найдено.\n\n"+reminderListText(reminders, b.location()),
		reminderListKeyboard(reminders),
	)
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
		"Напоминание удалено.\n\n"+reminderListText(reminders, b.location()),
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

	location := b.location()

	month, err := time.ParseInLocation("2006-01", value, location)
	if err != nil {
		return false
	}

	nowMonth := firstDayOfMonth(b.now())
	if firstDayOfMonth(month).Before(nowMonth) {
		draft.notice = "Эта дата уже прошла. Выберите будущую дату."
		draft.screen = draftScreenCalendar
		b.draftByChat[chatID] = draft

		return true
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

	location := b.location()

	date, err := time.ParseInLocation(time.DateOnly, value, location)
	if err != nil {
		return false
	}

	now := b.now()
	if date.Before(beginningOfDay(now)) {
		draft.notice = "Эта дата уже прошла. Выберите сегодняшнюю или будущую дату."
		draft.screen = draftScreenCalendar
		b.draftByChat[chatID] = draft

		return true
	}

	draft.date = date

	draft.calendarMonth = firstDayOfMonth(date)
	if !draft.remindAt(location).After(now) {
		next := nextReminderTime(now)
		draft.hour = next.Hour()
		draft.minute = next.Minute()
	}

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

	if !draft.remindAt(b.location()).After(b.now()) {
		draft.notice = "Это время уже прошло. Выберите будущее время."
	} else {
		draft.notice = ""
	}

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

	selectedTime := time.Date(2000, 1, 1, draft.hour, draft.minute, 0, 0, b.location())
	shiftedTime := selectedTime.Add(time.Duration(minutes) * time.Minute)
	draft.hour = shiftedTime.Hour()
	draft.minute = shiftedTime.Minute()

	if !draft.remindAt(b.location()).After(b.now()) {
		draft.notice = "Это время уже прошло. Выберите будущее время."
	} else {
		draft.notice = ""
	}

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

func (b *ReminderBot) setDraftTimeNotice(chatID int64, notice string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.notice = notice
	draft.waitingFor = ""
	draft.screen = draftScreenTime
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftScreen(chatID int64, screen draftScreen) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	if draft.calendarMonth.IsZero() {
		draft.calendarMonth = firstDayOfMonth(b.now())
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

func (b *ReminderBot) location() *time.Location {
	return time.Local
}

func (b *ReminderBot) now() time.Time {
	return time.Now().In(b.location())
}

func newReminderDraft(now time.Time) reminderDraft {
	next := nextReminderTime(now)

	return reminderDraft{
		hour:          next.Hour(),
		minute:        next.Minute(),
		screen:        draftScreenMain,
		calendarMonth: firstDayOfMonth(next),
	}
}

func nextReminderTime(now time.Time) time.Time {
	next := now.Add(time.Hour)

	minute := ((next.Minute() + 4) / 5) * 5
	if minute >= 60 {
		next = next.Add(time.Hour)
		minute = 0
	}

	return time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), minute, 0, 0, next.Location())
}

func (d reminderDraft) remindAt(location *time.Location) time.Time {
	if d.date.IsZero() {
		return time.Time{}
	}

	return time.Date(d.date.Year(), d.date.Month(), d.date.Day(), d.hour, d.minute, 0, 0, location)
}

func (d reminderDraft) selectedTimeText() string {
	return fmt.Sprintf("%02d:%02d", d.hour, d.minute)
}
