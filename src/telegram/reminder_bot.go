package telegram

import (
	"context"
	"errors"
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

var ErrUnsupportedReminderRepeat = errors.New("unsupported reminder repeat")
var errUnsupportedDeliveredReminderStatus = errors.New("unsupported delivered reminder status")

const draftPastTimeNotice = "Выбранное время уже прошло. Выберите будущее время."

func NewReminderBot(sender *Sender, service ReminderService, cfg config.Telegram) *ReminderBot {
	return &ReminderBot{
		bot:            sender.bot,
		cfg:            cfg,
		service:        service,
		log:            logger.Get("telegram/reminder-bot"),
		draftByChat:    make(map[int64]reminderDraft),
		uiByChat:       make(map[int64]int),
		locationByChat: make(map[int64]*time.Location),
	}
}

type ReminderBot struct {
	bot     *telego.Bot
	cfg     config.Telegram
	service ReminderService
	log     logger.Logger

	mu             sync.Mutex
	locationMu     sync.RWMutex
	draftByChat    map[int64]reminderDraft
	uiByChat       map[int64]int
	locationByChat map[int64]*time.Location
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
	CreateIntervalReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
		intervalDays int,
	) (database.ReminderView, error)
	CreateWeekdaysReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
		weekdaysMask int,
	) (database.ReminderView, error)
	CreateCustomDatesReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		selectedDates []time.Time,
		hour int,
		minute int,
		location *time.Location,
	) (database.ReminderView, error)
	Reminders(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error)
	ReminderHistory(ctx context.Context, telegramChatID int64) ([]database.ReminderView, error)
	DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error
	CompleteReminder(ctx context.Context, reminderID int64) error
	CancelDeliveredReminder(ctx context.Context, reminderID int64) error
	UserSettings(ctx context.Context, telegramChatID int64) (database.UserSettings, error)
	SetUserTimezoneByCity(ctx context.Context, telegramChatID int64, city string) error
	SetIntrusiveSettings(ctx context.Context, telegramChatID int64, enabled bool, interval time.Duration) error
}

type reminderDraft struct {
	title         string
	description   string
	notice        string
	date          time.Time
	hour          int
	minute        int
	repeat        reminderRepeat
	customDates   []time.Time
	weekdaysMask  int
	intervalDays  int
	screen        draftScreen
	calendarMonth time.Time
	waitingFor    draftField
}

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

	if handled, err := b.handleKnownMessage(ctx, chatID, message.MessageID, text); handled || err != nil {
		if err != nil {
			return err
		}

		return nil
	}

	if b.applyDraftText(ctx, chatID, text) {
		if b.draftExists(chatID) {
			if err := b.editDraft(ctx, chatID, b.currentUIMessageID(chatID)); err != nil {
				return err
			}
		} else if err := b.sendSettings(ctx, chatID); err != nil {
			return err
		}

		b.deleteMessage(ctx, chatID, message.MessageID)

		return nil
	}

	if err := b.replaceUI(
		ctx,
		chatID,
		"Не понял команду. Используйте основную клавиатуру.",
		mainKeyboard(),
	); err != nil {
		return err
	}

	b.deleteMessage(ctx, chatID, message.MessageID)

	return nil
}

func (b *ReminderBot) handleKnownMessage(
	ctx context.Context,
	chatID int64,
	messageID int,
	text string,
) (bool, error) {
	err, handled := b.runKnownMessageCommand(ctx, chatID, text)
	if !handled {
		return false, nil
	}

	if err != nil {
		return true, err
	}

	if shouldDeleteKnownMessage(text) {
		b.deleteMessage(ctx, chatID, messageID)
	}

	return true, nil
}

func (b *ReminderBot) runKnownMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case "/start", buttonStart:
		return b.sendStart(ctx, chatID), true
	case buttonCreateReminder:
		return b.startReminderDraft(ctx, chatID), true
	case buttonReminders:
		return b.sendReminders(ctx, chatID), true
	case buttonSettings:
		return b.sendSettings(ctx, chatID), true
	default:
		return nil, false
	}
}

func shouldDeleteKnownMessage(text string) bool {
	return text == buttonCreateReminder ||
		text == buttonReminders ||
		text == buttonSettings
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

	if handled, err := b.handleSettingsCallback(ctx, chatID, messageID, query.Data); handled {
		return err
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
	case data == callbackReminderHistory:
		return b.editReminderHistory(ctx, chatID, messageID)
	case strings.HasPrefix(data, callbackReminderDone):
		return b.acknowledgeReminder(
			ctx,
			chatID,
			messageID,
			data,
			callbackReminderDone,
			database.ReminderStatusCompleted,
		)
	case strings.HasPrefix(data, callbackReminderCancelDelivered):
		return b.acknowledgeReminder(
			ctx,
			chatID,
			messageID,
			data,
			callbackReminderCancelDelivered,
			database.ReminderStatusCanceled,
		)
	case strings.HasPrefix(data, callbackDeletePrefix):
		return b.deleteReminder(ctx, chatID, messageID, data)
	case strings.HasPrefix(data, callbackViewPrefix):
		return b.viewReminder(ctx, chatID, messageID, data)
	default:
		return nil
	}
}

func (b *ReminderBot) handleSettingsCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch data {
	case callbackSettings:
		return true, b.editSettings(ctx, chatID, messageID)
	case callbackSettingsLanguage:
		return true, b.editUI(ctx, chatID, messageID, "Язык приложения: русский", settingsKeyboard())
	case callbackSettingsTimezone:
		b.setDraftWaitingFor(chatID, fieldTimezoneCity)

		return true, b.editUI(ctx, chatID, messageID, "Введите город, чтобы определить часовой пояс.", nil)
	case callbackSettingsIntrusive:
		b.setDraftWaitingFor(chatID, fieldIntrusiveInterval)

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	case callbackSettingsIntrusiveOn:
		if err := b.service.SetIntrusiveSettings(ctx, chatID, true, 30*time.Minute); err != nil {
			return true, fmt.Errorf("enable intrusive reminders: %w", err)
		}

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	case callbackSettingsIntrusiveOff:
		if err := b.service.SetIntrusiveSettings(ctx, chatID, false, 30*time.Minute); err != nil {
			return true, fmt.Errorf("disable intrusive reminders: %w", err)
		}

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	default:
		return false, nil
	}
}

func (b *ReminderBot) handleDraftCommandCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	if handled, err := b.handleDraftDoneCallback(ctx, chatID, messageID, data); handled {
		return true, err
	}

	if handled, err := b.handleDraftNavigationCallback(ctx, chatID, messageID, data); handled {
		return true, err
	}

	return b.handleDraftFinalCommandCallback(ctx, chatID, messageID, data)
}

func (b *ReminderBot) handleDraftNavigationCallback(
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
	case callbackDraftRepeat:
		b.setDraftScreen(chatID, draftScreenRepeat)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftBack:
		b.setDraftScreen(chatID, draftScreenMain)

		return true, b.editDraft(ctx, chatID, messageID)
	default:
		return false, nil
	}
}

func (b *ReminderBot) handleDraftDoneCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch data {
	case callbackDraftCustomDone:
		b.setDraftScreen(chatID, draftScreenTime)

		return true, b.editDraft(ctx, chatID, messageID)
	case callbackDraftWeekdaysDone, callbackDraftIntervalDone:
		b.setDraftScreen(chatID, draftScreenMain)

		return true, b.editDraft(ctx, chatID, messageID)
	default:
		return false, nil
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

		return true, b.replaceUI(
			ctx,
			chatID,
			"Создание напоминания отменено.\n\n"+mainMenuText(),
			mainKeyboard(),
		)
	default:
		return false, nil
	}
}

func (b *ReminderBot) sendStart(ctx context.Context, chatID int64) error {
	return b.replaceUI(ctx, chatID, mainMenuText(), mainKeyboard())
}

func (b *ReminderBot) sendReminders(ctx context.Context, chatID int64) error {
	return b.replaceUI(ctx, chatID, remindersText(), remindersKeyboard())
}

func (b *ReminderBot) startReminderDraft(ctx context.Context, chatID int64) error {
	b.mu.Lock()
	b.draftByChat[chatID] = newReminderDraft(b.now(chatID))
	b.mu.Unlock()

	return b.replaceDraft(ctx, chatID)
}

func (b *ReminderBot) startReminderDraftInMessage(ctx context.Context, chatID int64, messageID int) error {
	b.mu.Lock()
	b.draftByChat[chatID] = newReminderDraft(b.now(chatID))
	b.mu.Unlock()

	return b.editDraft(ctx, chatID, messageID)
}

func (b *ReminderBot) replaceDraft(ctx context.Context, chatID int64) error {
	draft := b.draft(chatID)

	return b.replaceUI(ctx, chatID, draftText(draft, b.location(chatID)), draftKeyboard(draft, b.now(chatID)))
}

func (b *ReminderBot) editDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)

	return b.editUI(ctx, chatID, messageID, draftText(draft, b.location(chatID)), draftKeyboard(draft, b.now(chatID)))
}

func (b *ReminderBot) sendReminderList(ctx context.Context, chatID int64) error {
	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	return b.replaceUI(ctx, chatID, reminderListText(reminders), reminderListKeyboard(reminders))
}

func (b *ReminderBot) editReminderList(ctx context.Context, chatID int64, messageID int) error {
	reminders, err := b.service.Reminders(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	return b.editUI(ctx, chatID, messageID, reminderListText(reminders), reminderListKeyboard(reminders))
}

func (b *ReminderBot) sendReminderHistory(ctx context.Context, chatID int64) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	reminders, err := b.service.ReminderHistory(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminder history: %w", err)
	}

	return b.replaceUI(ctx, chatID, reminderHistoryText(reminders, b.location(chatID)), mainKeyboard())
}

func (b *ReminderBot) editReminderHistory(ctx context.Context, chatID int64, messageID int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	reminders, err := b.service.ReminderHistory(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminder history: %w", err)
	}

	return b.editUI(ctx, chatID, messageID, reminderHistoryText(reminders, b.location(chatID)), nil)
}

func (b *ReminderBot) sendSettings(ctx context.Context, chatID int64) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.replaceUI(ctx, chatID, settingsText(settings), settingsKeyboard())
}

func (b *ReminderBot) editSettings(ctx context.Context, chatID int64, messageID int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.editUI(ctx, chatID, messageID, settingsText(settings), settingsKeyboard())
}

func (b *ReminderBot) editIntrusiveSettings(ctx context.Context, chatID int64, messageID int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.editUI(ctx, chatID, messageID, intrusiveSettingsText(settings), intrusiveSettingsKeyboard())
}

func (b *ReminderBot) saveDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)
	remindAt := b.draftReminderTime(chatID, draft)

	if b.setDraftValidationNotice(chatID, draft, remindAt) {
		return b.editDraft(ctx, chatID, messageID)
	}

	reminder, err := b.createDraftReminder(ctx, chatID, draft, remindAt)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}

	b.deleteDraft(chatID)

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"Напоминание сохранено.\n\n"+reminderViewText(reminder, b.location(chatID)),
		reminderCreatedKeyboard(),
	)
}

func (b *ReminderBot) draftReminderTime(chatID int64, draft reminderDraft) time.Time {
	if draft.repeat != reminderRepeatCustomDates || len(draft.customDates) == 0 {
		return draft.remindAt(b.location(chatID))
	}

	firstDate := draft.customDates[0]
	for _, date := range draft.customDates[1:] {
		if date.Before(firstDate) {
			firstDate = date
		}
	}

	return time.Date(
		firstDate.Year(),
		firstDate.Month(),
		firstDate.Day(),
		draft.hour,
		draft.minute,
		0,
		0,
		b.location(chatID),
	)
}

func (b *ReminderBot) setDraftValidationNotice(chatID int64, draft reminderDraft, remindAt time.Time) bool {
	if draft.title == "" || remindAt.IsZero() {
		b.setDraftNotice(chatID, "Чтобы сохранить напоминание, добавьте название и дату с временем.")

		return true
	}

	if draft.repeat == reminderRepeatCustomWeekdays && draft.weekdaysMask == 0 {
		b.setDraftScreenNotice(chatID, draftScreenWeekdays, "Выберите хотя бы один день недели.")

		return true
	}

	if draft.repeat == reminderRepeatCustomInterval && draft.intervalDays <= 0 {
		b.setDraftScreenNotice(chatID, draftScreenInterval, "Введите интервал в днях: 1, 2, 3 ...")

		return true
	}

	if !remindAt.After(b.now(chatID)) {
		b.setDraftTimeNotice(chatID, "Это время уже прошло. Выберите будущую дату и время.")

		return true
	}

	return false
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
				reminderViewText(reminder, b.location(chatID)),
				reminderDetailsKeyboard(reminder),
			)
		}
	}

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"Напоминание не найдено.\n\n"+reminderListText(reminders),
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
		"Напоминание удалено.\n\n"+reminderListText(reminders),
		reminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) acknowledgeReminder(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
	prefix string,
	status database.ReminderStatus,
) error {
	reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, prefix), 10, 64)
	if err != nil {
		return fmt.Errorf("parse delivered reminder id: %w", err)
	}

	switch status {
	case database.ReminderStatusCompleted:
		err = b.service.CompleteReminder(ctx, reminderID)
	case database.ReminderStatusCanceled:
		err = b.service.CancelDeliveredReminder(ctx, reminderID)
	default:
		err = fmt.Errorf("%w: %s", errUnsupportedDeliveredReminderStatus, status)
	}

	if err != nil {
		return fmt.Errorf("acknowledge delivered reminder: %w", err)
	}

	return b.editUI(ctx, chatID, messageID, "Статус напоминания: "+telegramStatusText(status), nil)
}

func telegramStatusText(status database.ReminderStatus) string {
	switch status {
	case database.ReminderStatusCompleted:
		return "Выполнено"
	case database.ReminderStatusCanceled:
		return "Отменено"
	default:
		return string(status)
	}
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

func (b *ReminderBot) applyDraftText(ctx context.Context, chatID int64, text string) bool {
	b.mu.Lock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		b.mu.Unlock()

		return false
	}

	switch draft.waitingFor {
	case fieldTitle:
		draft.title = text
	case fieldDescription:
		draft.description = text
	case fieldTimezoneCity:
		b.mu.Unlock()

		return b.applyTimezoneCity(ctx, chatID, text)
	case fieldIntrusiveInterval:
		b.mu.Unlock()

		return b.applyIntrusiveInterval(ctx, chatID, text)
	case fieldRepeatInterval:
		b.mu.Unlock()

		return b.applyDraftIntervalDays(chatID, text)
	default:
		if draft.screen != draftScreenTime {
			b.mu.Unlock()

			return false
		}

		if !applyDraftTimeText(&draft, text, b.location(chatID), b.now(chatID)) {
			b.mu.Unlock()

			return false
		}
	}

	draft.notice = ""
	draft.waitingFor = ""
	draft.screen = draftScreenMain
	b.draftByChat[chatID] = draft
	b.mu.Unlock()

	return true
}

func (b *ReminderBot) applyTimezoneCity(ctx context.Context, chatID int64, text string) bool {
	if err := b.service.SetUserTimezoneByCity(ctx, chatID, text); err != nil {
		b.setDraftNotice(chatID, "Не удалось определить часовой пояс по этому городу.")

		return true
	}

	if err := b.loadChatLocation(ctx, chatID); err != nil {
		b.setDraftNotice(chatID, "Не удалось загрузить часовой пояс.")

		return true
	}

	b.deleteDraft(chatID)

	return true
}

func (b *ReminderBot) applyIntrusiveInterval(ctx context.Context, chatID int64, text string) bool {
	minutes, err := strconv.Atoi(text)
	if err != nil {
		b.setDraftNotice(chatID, "Введите интервал в минутах от 30 до 1440.")

		return true
	}

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		b.setDraftNotice(chatID, "Не удалось загрузить настройки.")

		return true
	}

	if err = b.service.SetIntrusiveSettings(
		ctx,
		chatID,
		settings.IntrusiveEnabled,
		time.Duration(minutes)*time.Minute,
	); err != nil {
		b.setDraftNotice(chatID, "Введите интервал в минутах от 30 до 1440.")

		return true
	}

	b.deleteDraft(chatID)

	return true
}

func (b *ReminderBot) applyDraftIntervalDays(chatID int64, text string) bool {
	days, err := strconv.Atoi(text)
	if err != nil || days <= 0 {
		b.setDraftScreenNotice(chatID, draftScreenInterval, "Введите интервал в днях: 1, 2, 3 ...")

		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.intervalDays = days
	draft.repeat = reminderRepeatCustomInterval

	draft.waitingFor = ""
	draft.notice = ""
	draft.screen = draftScreenMain
	b.draftByChat[chatID] = draft

	return true
}

func applyDraftTimeText(draft *reminderDraft, text string, location *time.Location, now time.Time) bool {
	parsed, err := time.Parse("15:04", text)
	if err != nil {
		return false
	}

	draft.hour = parsed.Hour()
	draft.minute = parsed.Minute()

	if !draft.remindAt(location).After(now) {
		draft.notice = draftPastTimeNotice
		draft.screen = draftScreenTime
	} else {
		draft.notice = ""
		draft.screen = draftScreenMain
	}

	draft.waitingFor = ""

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

	if value, ok := strings.CutPrefix(data, callbackRepeatPrefix); ok {
		return b.applyDraftRepeat(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, callbackWeekdayPrefix); ok {
		return b.applyDraftWeekday(chatID, value)
	}

	return false
}

func (b *ReminderBot) createDraftReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
	remindAt time.Time,
) (database.ReminderView, error) {
	switch draft.repeat {
	case reminderRepeatNone:
		return b.createDraftOneTimeReminder(ctx, chatID, draft, remindAt)
	case reminderRepeatWeekdays:
		return b.createDraftWeekdaysReminder(ctx, chatID, draft, remindAt, database.WeekdaysMaskWorkdays)
	case reminderRepeatCustomWeekdays:
		return b.createDraftWeekdaysReminder(ctx, chatID, draft, remindAt, draft.weekdaysMask)
	case reminderRepeatCustomDates:
		return b.createDraftCustomDatesReminder(ctx, chatID, draft)
	case reminderRepeatCustomInterval:
		return b.createDraftIntervalReminder(ctx, chatID, draft, remindAt, draft.intervalDays)
	default:
		intervalDays, ok := intervalDaysForRepeat(draft.repeat)
		if !ok {
			return database.ReminderView{}, fmt.Errorf("%w: %s", ErrUnsupportedReminderRepeat, draft.repeat)
		}

		return b.createDraftIntervalReminder(ctx, chatID, draft, remindAt, intervalDays)
	}
}

func (b *ReminderBot) createDraftOneTimeReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
	remindAt time.Time,
) (database.ReminderView, error) {
	reminder, err := b.service.CreateReminder(ctx, chatID, draft.title, draft.description, remindAt)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create one-time reminder: %w", err)
	}

	return reminder, nil
}

func (b *ReminderBot) createDraftWeekdaysReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
	remindAt time.Time,
	weekdaysMask int,
) (database.ReminderView, error) {
	reminder, err := b.service.CreateWeekdaysReminder(
		ctx,
		chatID,
		draft.title,
		draft.description,
		remindAt,
		weekdaysMask,
	)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create weekdays reminder: %w", err)
	}

	return reminder, nil
}

func (b *ReminderBot) createDraftCustomDatesReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
) (database.ReminderView, error) {
	reminder, err := b.service.CreateCustomDatesReminder(
		ctx,
		chatID,
		draft.title,
		draft.description,
		draft.customDates,
		draft.hour,
		draft.minute,
		b.location(chatID),
	)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create custom dates reminder: %w", err)
	}

	return reminder, nil
}

func intervalDaysForRepeat(repeat reminderRepeat) (int, bool) {
	switch repeat {
	case reminderRepeatEveryDay:
		return 1, true
	case reminderRepeatEveryTwoDays:
		return 2, true
	case reminderRepeatEveryWeek:
		return 7, true
	default:
		return 0, false
	}
}

func (b *ReminderBot) createDraftIntervalReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
	remindAt time.Time,
	intervalDays int,
) (database.ReminderView, error) {
	reminder, err := b.service.CreateIntervalReminder(
		ctx,
		chatID,
		draft.title,
		draft.description,
		remindAt,
		intervalDays,
	)
	if err != nil {
		return database.ReminderView{}, fmt.Errorf("create interval reminder: %w", err)
	}

	return reminder, nil
}

func (b *ReminderBot) applyDraftMonth(chatID int64, value string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	location := b.location(chatID)

	month, err := time.ParseInLocation("2006-01", value, location)
	if err != nil {
		return false
	}

	nowMonth := firstDayOfMonth(b.now(chatID))
	if firstDayOfMonth(month).Before(nowMonth) {
		draft.notice = "Нельзя выбрать месяц в прошлом. Выберите текущий или будущий месяц."
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

func (b *ReminderBot) applyDraftRepeat(chatID int64, value string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	draft.repeat = reminderRepeat(value)
	draft.notice = ""

	switch draft.repeat {
	case reminderRepeatCustomDates:
		draft.screen = draftScreenCalendar
	case reminderRepeatCustomWeekdays:
		draft.screen = draftScreenWeekdays
	case reminderRepeatCustomInterval:
		if draft.intervalDays == 0 {
			draft.intervalDays = 1
		}

		draft.waitingFor = fieldRepeatInterval
		draft.screen = draftScreenInterval
	default:
		draft.screen = draftScreenMain
	}

	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) applyDraftWeekday(chatID int64, value string) bool {
	mask, err := strconv.Atoi(value)
	if err != nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	draft.weekdaysMask ^= mask
	draft.repeat = reminderRepeatCustomWeekdays
	draft.notice = ""
	draft.screen = draftScreenWeekdays
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

	location := b.location(chatID)

	date, err := time.ParseInLocation(time.DateOnly, value, location)
	if err != nil {
		return false
	}

	now := b.now(chatID)
	if date.Before(beginningOfDay(now)) {
		draft.notice = "Нельзя выбрать дату в прошлом. Выберите сегодняшнюю или будущую дату."
		draft.screen = draftScreenCalendar
		b.draftByChat[chatID] = draft

		return true
	}

	if draft.repeat == reminderRepeatCustomDates {
		draft.customDates = toggleDraftCustomDate(draft.customDates, date)
		draft.calendarMonth = firstDayOfMonth(date)
		draft.notice = ""
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

func toggleDraftCustomDate(dates []time.Time, date time.Time) []time.Time {
	for index, selected := range dates {
		if sameDraftDate(selected, date) {
			return append(dates[:index], dates[index+1:]...)
		}
	}

	return append(dates, date)
}

func sameDraftDate(left time.Time, right time.Time) bool {
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
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

	if !draft.remindAt(b.location(chatID)).After(b.now(chatID)) {
		draft.notice = draftPastTimeNotice
	} else {
		draft.notice = ""
	}

	draft.screen = draftScreenMain
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

	if draft.screen == draftScreenInterval {
		days := draft.intervalDays
		if days == 0 {
			days = 1
		}

		days += minutes / 1440
		if days < 1 {
			days = 1
		}

		draft.intervalDays = days
		draft.repeat = reminderRepeatCustomInterval
		draft.notice = ""
		draft.screen = draftScreenInterval
		b.draftByChat[chatID] = draft

		return true
	}

	selectedTime := time.Date(2000, 1, 1, draft.hour, draft.minute, 0, 0, b.location(chatID))
	shiftedTime := selectedTime.Add(time.Duration(minutes) * time.Minute)
	draft.hour = shiftedTime.Hour()
	draft.minute = shiftedTime.Minute()

	if !draft.remindAt(b.location(chatID)).After(b.now(chatID)) {
		draft.notice = draftPastTimeNotice
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

func (b *ReminderBot) setDraftScreenNotice(chatID int64, screen draftScreen, notice string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.notice = notice
	draft.waitingFor = ""
	draft.screen = screen
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftScreen(chatID int64, screen draftScreen) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	if draft.calendarMonth.IsZero() {
		draft.calendarMonth = firstDayOfMonth(b.now(chatID))
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

func (b *ReminderBot) draftExists(chatID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, exists := b.draftByChat[chatID]

	return exists
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

func (b *ReminderBot) loadChatLocation(ctx context.Context, chatID int64) error {
	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load chat location settings: %w", err)
	}

	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		location = time.Local
	}

	b.setChatLocation(chatID, location)

	return nil
}

func (b *ReminderBot) setChatLocation(chatID int64, location *time.Location) {
	b.locationMu.Lock()
	defer b.locationMu.Unlock()

	if b.locationByChat == nil {
		b.locationByChat = make(map[int64]*time.Location)
	}

	b.locationByChat[chatID] = location
}

func (b *ReminderBot) location(chatID int64) *time.Location {
	b.locationMu.RLock()
	defer b.locationMu.RUnlock()

	location := b.locationByChat[chatID]
	if location == nil {
		return time.Local
	}

	return location
}

func (b *ReminderBot) now(chatID int64) time.Time {
	return time.Now().In(b.location(chatID))
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
