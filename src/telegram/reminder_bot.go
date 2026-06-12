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
	telegramui "github.com/EgorKo25/EasyNote/src/telegram/ui"
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
		pageByChat:     make(map[int64]reminderPage),
		timezoneByChat: make(map[int64]int),
		menuByChat:     make(map[int64]chatMenu),
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
	pageByChat     map[int64]reminderPage
	timezoneByChat map[int64]int
	menuByChat     map[int64]chatMenu
}

type reminderPage struct {
	kind   reminderPageKind
	offset int
}

type reminderPageKind string

const (
	reminderPageSize                     = 2
	reminderPageActive  reminderPageKind = "active"
	reminderPageHistory reminderPageKind = "history"
)

type chatMenu string

const (
	chatMenuMain              chatMenu = "main"
	chatMenuCreate            chatMenu = "create"
	chatMenuReminders         chatMenu = "reminders"
	chatMenuReminderList      chatMenu = "reminder_list"
	chatMenuSettings          chatMenu = "settings"
	chatMenuTimezone          chatMenu = "timezone"
	chatMenuIntrusive         chatMenu = "intrusive"
	chatMenuIntrusiveInterval chatMenu = "intrusive_interval"
)

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
	Reminders(ctx context.Context, telegramChatID int64, limit int, offset int) ([]database.ReminderView, error)
	ReminderHistory(ctx context.Context, telegramChatID int64, limit int, offset int) ([]database.ReminderView, error)
	DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error
	CompleteOccurrence(
		ctx context.Context,
		reminderID int64,
		status database.ReminderStatus,
		sentAt *time.Time,
	) error
	UserSettings(ctx context.Context, telegramChatID int64) (database.UserSettings, error)
	SetUserTimezone(ctx context.Context, telegramChatID int64, timezone string) error
	SetIntrusiveSettings(ctx context.Context, telegramChatID int64, enabled bool, interval time.Duration) error
}

type reminderDraft struct {
	title         string
	description   string
	notice        string
	date          time.Time
	hour          int
	minute        int
	repeat        telegramui.Repeat
	customDates   []time.Time
	weekdaysMask  int
	intervalDays  int
	screen        telegramui.Screen
	calendarMonth time.Time
	waitingFor    telegramui.Field
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
		telegramui.MainKeyboard(),
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
	if err, handled := b.runMainMessageCommand(ctx, chatID, text); handled {
		return err, true
	}

	if err, handled := b.runDraftMessageCommand(ctx, chatID, text); handled {
		return err, true
	}

	if err, handled := b.runSettingsMessageCommand(ctx, chatID, text); handled {
		return err, true
	}

	if handled, err := b.handleSelectedReplyMessage(ctx, chatID, text); handled || err != nil {
		return err, handled
	}

	return nil, false
}

func (b *ReminderBot) runMainMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	if err, handled := b.runTopLevelMessageCommand(ctx, chatID, text); handled {
		return err, true
	}

	return b.runReminderListMessageCommand(ctx, chatID, text)
}

func (b *ReminderBot) runTopLevelMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case "/start", telegramui.ButtonStart:
		return b.sendStart(ctx, chatID), true
	case telegramui.ButtonCreateReminder:
		return b.sendCreate(ctx, chatID), true
	case telegramui.ButtonCreateReminderKind:
		return b.startReminderDraft(ctx, chatID), true
	case telegramui.ButtonReminders:
		return b.sendReminders(ctx, chatID), true
	case telegramui.ButtonSettings:
		return b.sendSettings(ctx, chatID), true
	case telegramui.ButtonBack:
		return b.handleBackMessage(ctx, chatID), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runDraftMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	for _, handler := range []func(context.Context, int64, string) (error, bool){
		b.runDraftFieldMessageCommand,
		b.runDraftRepeatMessageCommand,
		b.runDraftTimeMessageCommand,
		b.runDraftDateMessageCommand,
	} {
		if err, handled := handler(ctx, chatID, text); handled {
			return err, true
		}
	}

	return nil, false
}

func (b *ReminderBot) runReminderListMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case telegramui.ButtonReminderList:
		return b.sendReminderListPage(ctx, chatID, 0), true
	case telegramui.ButtonReminderHistory:
		return b.sendReminderHistoryPage(ctx, chatID, 0), true
	case telegramui.ButtonPreviousPage:
		return b.sendShiftedReminderPage(ctx, chatID, -reminderPageSize), true
	case telegramui.ButtonNextPage:
		return b.sendShiftedReminderPage(ctx, chatID, reminderPageSize), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runDraftFieldMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case telegramui.ButtonDraftTitle, telegramui.ButtonDraftEditTitle:
		b.setDraftWaitingFor(chatID, telegramui.FieldTitle)

		return b.replaceDraft(ctx, chatID), true
	case telegramui.ButtonDraftDescription, telegramui.ButtonDraftEditDescription:
		b.setDraftWaitingFor(chatID, telegramui.FieldDescription)

		return b.replaceDraft(ctx, chatID), true
	case telegramui.ButtonDraftDate:
		b.setDraftScreen(chatID, telegramui.ScreenDate)

		return b.replaceDraft(ctx, chatID), true
	case telegramui.ButtonDraftRepeat:
		b.setDraftScreen(chatID, telegramui.ScreenRepeat)

		return b.replaceDraft(ctx, chatID), true
	case telegramui.ButtonDraftSave:
		return b.saveDraft(ctx, chatID, b.currentUIMessageID(chatID)), true
	case telegramui.ButtonDraftCancel:
		b.deleteDraft(chatID)

		return b.sendStart(ctx, chatID), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runDraftRepeatMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case telegramui.ButtonRepeatEveryDay:
		b.applyDraftRepeat(chatID, string(telegramui.RepeatEveryDay))

		return b.replaceDraft(ctx, chatID), true
	case telegramui.ButtonRepeatWeekdays:
		b.applyDraftRepeat(chatID, string(telegramui.RepeatWeekdays))

		return b.replaceDraft(ctx, chatID), true
	case telegramui.ButtonRepeatCustomWeekdays:
		b.applyDraftRepeat(chatID, string(telegramui.RepeatCustomWeekdays))

		return b.replaceDraft(ctx, chatID), true
	case telegramui.ButtonRepeatInterval:
		b.applyDraftRepeat(chatID, string(telegramui.RepeatCustomInterval))

		return b.replaceDraft(ctx, chatID), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runDraftTimeMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case telegramui.ButtonDone:
		return b.handleDoneMessage(ctx, chatID), true
	case telegramui.ButtonMinusHour:
		return b.handleTimeShiftMessage(ctx, chatID, -60), true
	case telegramui.ButtonPlusHour:
		return b.handleTimeShiftMessage(ctx, chatID, 60), true
	case telegramui.ButtonMinusFiveMinutes:
		return b.handleTimeShiftMessage(ctx, chatID, -5), true
	case telegramui.ButtonPlusFiveMinutes:
		return b.handleTimeShiftMessage(ctx, chatID, 5), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runDraftDateMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case "Сегодня":
		b.applyDraftDate(chatID, b.now(chatID).Format(time.DateOnly))

		return b.replaceDraft(ctx, chatID), true
	case "Завтра":
		b.applyDraftDate(chatID, b.now(chatID).AddDate(0, 0, 1).Format(time.DateOnly))

		return b.replaceDraft(ctx, chatID), true
	case "Выбрать дату":
		b.setDraftScreen(chatID, telegramui.ScreenCalendar)

		return b.replaceDraft(ctx, chatID), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runSettingsMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case telegramui.ButtonSettingsLanguage:
		return b.replaceUI(ctx, chatID, "Язык приложения: русский", telegramui.SettingsKeyboard()), true
	case telegramui.ButtonSettingsTimezone:
		return b.sendTimezoneSettings(ctx, chatID), true
	case telegramui.ButtonSettingsIntrusive:
		return b.sendIntrusiveSettings(ctx, chatID), true
	case telegramui.ButtonIntrusiveEnabled, telegramui.ButtonIntrusiveDisabled:
		return b.toggleIntrusiveSettings(ctx, chatID), true
	case telegramui.ButtonIntrusiveInterval:
		return b.sendIntrusiveInterval(ctx, chatID), true
	default:
		return nil, false
	}
}

func shouldDeleteKnownMessage(text string) bool {
	return text == telegramui.ButtonCreateReminder ||
		text == telegramui.ButtonReminders ||
		text == telegramui.ButtonSettings
}

func (b *ReminderBot) handleCallback(ctx context.Context, query telego.CallbackQuery) error {
	if query.Message == nil {
		return nil
	}

	chatID := query.Message.GetChat().ID
	if err := b.bot.AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID)); err != nil {
		return fmt.Errorf("answer callback: %w", err)
	}

	if query.Data == telegramui.CallbackNoop {
		return nil
	}

	messageID := query.Message.GetMessageID()
	if query.Data == telegramui.CallbackMainMenu {
		return b.replaceUI(ctx, chatID, telegramui.MainMenuText(), telegramui.MainKeyboard())
	}

	if handled, err := b.handleSettingsCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if handled, err := b.handleDraftCommandCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if b.applyDraftDateCallback(chatID, query.Data) {
		if b.draft(chatID).screen != telegramui.ScreenCalendar {
			return b.replaceDraft(ctx, chatID)
		}

		return b.editDraft(ctx, chatID, messageID)
	}

	return b.handleReminderCallback(ctx, chatID, messageID, query.Data)
}

func (b *ReminderBot) handleReminderCallback(ctx context.Context, chatID int64, messageID int, data string) error {
	switch {
	case data == telegramui.CallbackCreateReminder:
		return b.startReminderDraftInMessage(ctx, chatID, messageID)
	case data == telegramui.CallbackReminderList || data == telegramui.CallbackReminderBackList:
		return b.editReminderList(ctx, chatID, messageID)
	case data == telegramui.CallbackReminderHistory:
		return b.editReminderHistory(ctx, chatID, messageID)
	case strings.HasPrefix(data, telegramui.CallbackReminderDone):
		return b.acknowledgeReminder(
			ctx,
			chatID,
			messageID,
			data,
			telegramui.CallbackReminderDone,
			database.ReminderStatusCompleted,
		)
	case strings.HasPrefix(data, telegramui.CallbackReminderCancelDelivered):
		return b.acknowledgeReminder(
			ctx,
			chatID,
			messageID,
			data,
			telegramui.CallbackReminderCancelDelivered,
			database.ReminderStatusCanceled,
		)
	case strings.HasPrefix(data, telegramui.CallbackDeletePrefix):
		return b.deleteReminder(ctx, chatID, messageID, data)
	case strings.HasPrefix(data, telegramui.CallbackViewPrefix):
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
	if handled, err := b.handleTimezoneSettingsCallback(ctx, chatID, messageID, data); handled {
		return true, err
	}

	switch data {
	case telegramui.CallbackSettings:
		return true, b.editSettings(ctx, chatID, messageID)
	case telegramui.CallbackSettingsLanguage:
		return true, b.replaceUI(ctx, chatID, "Язык приложения: русский", telegramui.SettingsKeyboard())
	case telegramui.CallbackSettingsIntrusive:
		b.setDraftWaitingFor(chatID, telegramui.FieldIntrusiveInterval)

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	case telegramui.CallbackSettingsIntrusiveOn:
		if err := b.service.SetIntrusiveSettings(ctx, chatID, true, 30*time.Minute); err != nil {
			return true, fmt.Errorf("enable intrusive reminders: %w", err)
		}

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	case telegramui.CallbackSettingsIntrusiveOff:
		if err := b.service.SetIntrusiveSettings(ctx, chatID, false, 30*time.Minute); err != nil {
			return true, fmt.Errorf("disable intrusive reminders: %w", err)
		}

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	default:
		return false, nil
	}
}

func (b *ReminderBot) handleTimezoneSettingsCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch {
	case data == telegramui.CallbackSettingsTimezone:
		return true, b.editTimezoneSettings(ctx, chatID, messageID, data)
	case strings.HasPrefix(data, telegramui.CallbackSettingsTimezonePrev),
		strings.HasPrefix(data, telegramui.CallbackSettingsTimezoneNext):
		return true, b.editTimezoneSettings(ctx, chatID, messageID, data)
	case strings.HasPrefix(data, telegramui.CallbackSettingsTimezoneSelect):
		return true, b.setTimezoneByCallback(ctx, chatID, messageID, data)
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
	case telegramui.CallbackDraftTitle:
		b.setDraftWaitingFor(chatID, telegramui.FieldTitle)

		return true, b.editDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftDescription:
		b.setDraftWaitingFor(chatID, telegramui.FieldDescription)

		return true, b.editDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftDate:
		b.setDraftScreen(chatID, telegramui.ScreenDate)

		return true, b.editDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftCalendar:
		b.setDraftScreen(chatID, telegramui.ScreenCalendar)

		return true, b.editDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftRepeat:
		b.setDraftScreen(chatID, telegramui.ScreenRepeat)

		return true, b.editDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftBack:
		b.setDraftScreen(chatID, telegramui.ScreenMain)

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
	case telegramui.CallbackDraftCustomDone:
		b.setDraftScreen(chatID, telegramui.ScreenTime)

		return true, b.editDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftWeekdaysDone, telegramui.CallbackDraftIntervalDone:
		b.setDraftScreen(chatID, telegramui.ScreenMain)

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
	case telegramui.CallbackDraftSave:
		return true, b.saveDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftTimeDone:
		b.setDraftScreen(chatID, telegramui.ScreenMain)

		return true, b.editDraft(ctx, chatID, messageID)
	case telegramui.CallbackDraftCancel:
		b.deleteDraft(chatID)

		return true, b.replaceUI(
			ctx,
			chatID,
			"Создание напоминания отменено.\n\n"+telegramui.MainMenuText(),
			telegramui.MainKeyboard(),
		)
	default:
		return false, nil
	}
}

func (b *ReminderBot) sendStart(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuMain)

	return b.replaceUI(ctx, chatID, telegramui.MainMenuText(), telegramui.MainKeyboard())
}

func (b *ReminderBot) sendCreate(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuCreate)

	return b.replaceUI(ctx, chatID, telegramui.CreateText(), telegramui.CreateKeyboard())
}

func (b *ReminderBot) sendReminders(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuReminders)

	return b.replaceUI(ctx, chatID, telegramui.RemindersText(), telegramui.RemindersKeyboard())
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
	b.setChatMenu(chatID, chatMenuCreate)

	draft := b.draft(chatID)
	uiDraftValue := uiDraft(draft)

	var keyboard telego.ReplyMarkup = telegramui.DraftReplyKeyboard(uiDraftValue, b.now(chatID))

	if draft.screen == telegramui.ScreenCalendar {
		keyboard = telegramui.DraftKeyboard(uiDraftValue, b.now(chatID))
	}

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.DraftText(uiDraftValue, b.location(chatID)),
		keyboard,
	)
}

func (b *ReminderBot) editDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)

	return b.editUI(
		ctx,
		chatID,
		messageID,
		telegramui.DraftText(uiDraft(draft), b.location(chatID)),
		telegramui.DraftKeyboard(uiDraft(draft), b.now(chatID)),
	)
}

func (b *ReminderBot) sendReminderList(ctx context.Context, chatID int64) error {
	return b.sendReminderListPage(ctx, chatID, 0)
}

func (b *ReminderBot) editReminderList(ctx context.Context, chatID int64, messageID int) error {
	return b.sendReminderListPage(ctx, chatID, 0)
}

func (b *ReminderBot) sendReminderHistory(ctx context.Context, chatID int64) error {
	return b.sendReminderHistoryPage(ctx, chatID, 0)
}

func (b *ReminderBot) editReminderHistory(ctx context.Context, chatID int64, messageID int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	reminders, err := b.service.ReminderHistory(ctx, chatID, reminderPageSize, 0)
	if err != nil {
		return fmt.Errorf("load reminder history: %w", err)
	}

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.ReminderHistoryText(reminders),
		telegramui.ReminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) sendReminderListPage(ctx context.Context, chatID int64, offset int) error {
	offset = normalizePageOffset(offset)

	reminders, err := b.service.Reminders(ctx, chatID, reminderPageSize, offset)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	if len(reminders) == 0 && offset > 0 {
		return b.sendReminderListPage(ctx, chatID, offset-reminderPageSize)
	}

	b.setReminderPage(chatID, reminderPage{kind: reminderPageActive, offset: offset})
	b.setChatMenu(chatID, chatMenuReminderList)

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.ReminderListText(reminders),
		telegramui.ReminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) sendReminderHistoryPage(ctx context.Context, chatID int64, offset int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	offset = normalizePageOffset(offset)

	reminders, err := b.service.ReminderHistory(ctx, chatID, reminderPageSize, offset)
	if err != nil {
		return fmt.Errorf("load reminder history: %w", err)
	}

	if len(reminders) == 0 && offset > 0 {
		return b.sendReminderHistoryPage(ctx, chatID, offset-reminderPageSize)
	}

	b.setReminderPage(chatID, reminderPage{kind: reminderPageHistory, offset: offset})
	b.setChatMenu(chatID, chatMenuReminderList)

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.ReminderHistoryText(reminders),
		telegramui.ReminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) sendShiftedReminderPage(ctx context.Context, chatID int64, shift int) error {
	page := b.reminderPage(chatID)
	switch page.kind {
	case reminderPageHistory:
		return b.sendReminderHistoryPage(ctx, chatID, page.offset+shift)
	default:
		return b.sendReminderListPage(ctx, chatID, page.offset+shift)
	}
}

func normalizePageOffset(offset int) int {
	if offset < 0 {
		return 0
	}

	return offset
}

func (b *ReminderBot) sendSettings(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuSettings)

	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.replaceUI(ctx, chatID, telegramui.SettingsText(settings), telegramui.SettingsKeyboard())
}

func (b *ReminderBot) editSettings(ctx context.Context, chatID int64, messageID int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.replaceUI(ctx, chatID, telegramui.SettingsText(settings), telegramui.SettingsKeyboard())
}

func (b *ReminderBot) editIntrusiveSettings(ctx context.Context, chatID int64, messageID int) error {
	return b.sendIntrusiveSettings(ctx, chatID)
}

func (b *ReminderBot) sendIntrusiveSettings(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuIntrusive)

	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.IntrusiveSettingsText(settings),
		telegramui.IntrusiveSettingsKeyboard(settings.IntrusiveEnabled),
	)
}

func (b *ReminderBot) toggleIntrusiveSettings(ctx context.Context, chatID int64) error {
	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	if err = b.service.SetIntrusiveSettings(
		ctx,
		chatID,
		!settings.IntrusiveEnabled,
		settings.IntrusiveInterval,
	); err != nil {
		return fmt.Errorf("toggle intrusive reminders: %w", err)
	}

	return b.sendIntrusiveSettings(ctx, chatID)
}

func (b *ReminderBot) sendIntrusiveInterval(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuIntrusiveInterval)

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.IntrusiveIntervalText(settings.IntrusiveInterval),
		telegramui.IntrusiveIntervalKeyboard(settings.IntrusiveInterval),
	)
}

func (b *ReminderBot) shiftIntrusiveInterval(ctx context.Context, chatID int64, minutes int) error {
	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	interval := settings.IntrusiveInterval + time.Duration(minutes)*time.Minute
	interval = max(interval, time.Minute)
	interval = min(interval, 2*time.Hour)

	if err = b.service.SetIntrusiveSettings(ctx, chatID, settings.IntrusiveEnabled, interval); err != nil {
		return fmt.Errorf("set intrusive interval: %w", err)
	}

	return b.sendIntrusiveInterval(ctx, chatID)
}

func (b *ReminderBot) editTimezoneSettings(ctx context.Context, chatID int64, messageID int, data string) error {
	var index int

	if data == telegramui.CallbackSettingsTimezone {
		settings, err := b.service.UserSettings(ctx, chatID)
		if err != nil {
			return fmt.Errorf("load user settings: %w", err)
		}

		index = telegramui.TimezoneIndex(settings.Timezone)
	} else {
		parsedIndex, err := timezoneCallbackIndex(data)
		if err != nil {
			return err
		}

		index = parsedIndex
	}

	return b.replaceUI(
		ctx,
		chatID,
		"Выберите часовой пояс.",
		telegramui.TimezoneKeyboard(index, b.now(chatID)),
	)
}

func (b *ReminderBot) sendTimezoneSettings(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuTimezone)

	settings, err := b.service.UserSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	index := telegramui.TimezoneIndex(settings.Timezone)
	b.setTimezoneIndex(chatID, index)

	return b.replaceUI(ctx, chatID, "Выберите часовой пояс.", telegramui.TimezoneKeyboard(index, b.now(chatID)))
}

func (b *ReminderBot) shiftTimezone(ctx context.Context, chatID int64, shift int) error {
	index := telegramui.TimezoneIndexByShift(b.timezoneIndex(chatID), shift)
	b.setTimezoneIndex(chatID, index)

	return b.replaceUI(ctx, chatID, "Выберите часовой пояс.", telegramui.TimezoneKeyboard(index, b.now(chatID)))
}

func (b *ReminderBot) saveCurrentTimezone(ctx context.Context, chatID int64) error {
	option := telegramui.TimezoneByIndex(b.timezoneIndex(chatID))
	if err := b.service.SetUserTimezone(ctx, chatID, option.Timezone); err != nil {
		return fmt.Errorf("set user timezone: %w", err)
	}

	return b.sendSettings(ctx, chatID)
}

func (b *ReminderBot) setTimezoneByCallback(ctx context.Context, chatID int64, messageID int, data string) error {
	index, err := timezoneCallbackIndex(data)
	if err != nil {
		return err
	}

	option := telegramui.TimezoneByIndex(index)
	if err = b.service.SetUserTimezone(ctx, chatID, option.Timezone); err != nil {
		return fmt.Errorf("set user timezone: %w", err)
	}

	return b.editSettings(ctx, chatID, messageID)
}

func timezoneCallbackIndex(data string) (int, error) {
	value := data
	for _, prefix := range []string{
		telegramui.CallbackSettingsTimezonePrev,
		telegramui.CallbackSettingsTimezoneNext,
		telegramui.CallbackSettingsTimezoneSelect,
	} {
		if suffix, ok := strings.CutPrefix(data, prefix); ok {
			value = suffix

			break
		}
	}

	index, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse timezone index: %w", err)
	}

	return index, nil
}

func (b *ReminderBot) handleBackMessage(ctx context.Context, chatID int64) error {
	if b.draftExists(chatID) {
		draft := b.draft(chatID)
		if draft.screen != telegramui.ScreenMain {
			b.setDraftScreen(chatID, telegramui.ScreenMain)

			return b.replaceDraft(ctx, chatID)
		}

		b.deleteDraft(chatID)

		return b.sendCreate(ctx, chatID)
	}

	switch b.chatMenu(chatID) {
	case chatMenuTimezone:
		return b.saveCurrentTimezone(ctx, chatID)
	case chatMenuIntrusiveInterval:
		return b.sendIntrusiveSettings(ctx, chatID)
	case chatMenuIntrusive:
		return b.sendSettings(ctx, chatID)
	case chatMenuReminderList:
		return b.sendReminders(ctx, chatID)
	default:
		return b.sendStart(ctx, chatID)
	}
}

func (b *ReminderBot) handleDoneMessage(ctx context.Context, chatID int64) error {
	if b.chatMenu(chatID) == chatMenuIntrusiveInterval {
		return b.sendIntrusiveSettings(ctx, chatID)
	}

	if !b.draftExists(chatID) {
		return nil
	}

	draft := b.draft(chatID)
	if draftScreenCanBeClosed(draft.screen) {
		b.setDraftScreen(chatID, telegramui.ScreenMain)
	}

	return b.replaceDraft(ctx, chatID)
}

func draftScreenCanBeClosed(screen telegramui.Screen) bool {
	return screen == telegramui.ScreenTime ||
		screen == telegramui.ScreenWeekdays ||
		screen == telegramui.ScreenInterval
}

func (b *ReminderBot) handleTimeShiftMessage(ctx context.Context, chatID int64, minutes int) error {
	switch b.chatMenu(chatID) {
	case chatMenuTimezone:
		if minutes < 0 {
			return b.shiftTimezone(ctx, chatID, -1)
		}

		return b.shiftTimezone(ctx, chatID, 1)
	case chatMenuIntrusiveInterval:
		return b.shiftIntrusiveInterval(ctx, chatID, minutes)
	default:
		if b.applyDraftTimeShift(chatID, strconv.Itoa(minutes)) {
			return b.replaceDraft(ctx, chatID)
		}
	}

	return nil
}

func (b *ReminderBot) handleSelectedReplyMessage(ctx context.Context, chatID int64, text string) (bool, error) {
	if b.chatMenu(chatID) == chatMenuTimezone {
		if text == telegramui.ButtonPreviousPage {
			return true, b.shiftTimezone(ctx, chatID, -1)
		}

		if text == telegramui.ButtonNextPage {
			return true, b.shiftTimezone(ctx, chatID, 1)
		}

		return false, nil
	}

	if strings.HasPrefix(text, "#") {
		return true, b.viewReminderByReply(ctx, chatID, text)
	}

	if b.applyDraftTimeTextByMessage(chatID, text) {
		return true, b.replaceDraft(ctx, chatID)
	}

	return false, nil
}

func (b *ReminderBot) applyDraftTimeTextByMessage(chatID int64, text string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft, exists := b.draftByChat[chatID]
	if !exists || draft.screen != telegramui.ScreenTime {
		return false
	}

	if !applyDraftTimeText(&draft, text, b.location(chatID), b.now(chatID)) {
		return false
	}

	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) viewReminderByReply(ctx context.Context, chatID int64, text string) error {
	idText, _, _ := strings.Cut(strings.TrimPrefix(text, "#"), " ")

	reminderID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return fmt.Errorf("parse reminder id: %w", err)
	}

	page := b.reminderPage(chatID)

	var reminders []database.ReminderView

	if page.kind == reminderPageHistory {
		reminders, err = b.service.ReminderHistory(ctx, chatID, reminderPageSize, page.offset)
	} else {
		reminders, err = b.service.Reminders(ctx, chatID, reminderPageSize, page.offset)
	}

	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	for _, reminder := range reminders {
		if reminder.ID == reminderID {
			return b.replaceUI(
				ctx,
				chatID,
				telegramui.ReminderViewText(reminder, b.location(chatID)),
				telegramui.ReminderListKeyboard(reminders),
			)
		}
	}

	return b.sendShiftedReminderPage(ctx, chatID, 0)
}

func (b *ReminderBot) saveDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)
	remindAt := b.draftReminderTime(chatID, draft)

	if b.setDraftValidationNotice(chatID, draft, remindAt) {
		return b.replaceDraft(ctx, chatID)
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
		"Напоминание сохранено.\n\n"+telegramui.ReminderViewText(reminder, b.location(chatID)),
		telegramui.ReminderCreatedKeyboard(),
	)
}

func (b *ReminderBot) draftReminderTime(chatID int64, draft reminderDraft) time.Time {
	if draft.repeat != telegramui.RepeatCustomDates || len(draft.customDates) == 0 {
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

	if draft.repeat == telegramui.RepeatCustomWeekdays && draft.weekdaysMask == 0 {
		b.setDraftScreenNotice(chatID, telegramui.ScreenWeekdays, "Выберите хотя бы один день недели.")

		return true
	}

	if draft.repeat == telegramui.RepeatCustomInterval && draft.intervalDays <= 0 {
		b.setDraftScreenNotice(chatID, telegramui.ScreenInterval, "Введите интервал в днях: 1, 2, 3 ...")

		return true
	}

	if !remindAt.After(b.now(chatID)) {
		b.setDraftTimeNotice(chatID, "Это время уже прошло. Выберите будущую дату и время.")

		return true
	}

	return false
}

func (b *ReminderBot) viewReminder(ctx context.Context, chatID int64, messageID int, data string) error {
	reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, telegramui.CallbackViewPrefix), 10, 64)
	if err != nil {
		return fmt.Errorf("parse reminder id: %w", err)
	}

	page := b.reminderPage(chatID)

	reminders, err := b.service.Reminders(ctx, chatID, reminderPageSize, page.offset)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	for _, reminder := range reminders {
		if reminder.ID == reminderID {
			return b.editUI(
				ctx,
				chatID,
				messageID,
				telegramui.ReminderViewText(reminder, b.location(chatID)),
				telegramui.ReminderDetailsKeyboard(reminder),
			)
		}
	}

	return b.replaceUI(
		ctx,
		chatID,
		"Напоминание не найдено.\n\n"+telegramui.ReminderListText(reminders),
		telegramui.ReminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) deleteReminder(ctx context.Context, chatID int64, messageID int, data string) error {
	reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, telegramui.CallbackDeletePrefix), 10, 64)
	if err != nil {
		return fmt.Errorf("parse reminder id: %w", err)
	}

	if err = b.service.DeleteReminder(ctx, chatID, reminderID); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}

	page := b.reminderPage(chatID)

	reminders, err := b.service.Reminders(ctx, chatID, reminderPageSize, page.offset)
	if err != nil {
		return fmt.Errorf("load reminders after delete: %w", err)
	}

	return b.replaceUI(
		ctx,
		chatID,
		"Напоминание удалено.\n\n"+telegramui.ReminderListText(reminders),
		telegramui.ReminderListKeyboard(reminders),
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

	if status != database.ReminderStatusCompleted && status != database.ReminderStatusCanceled {
		return fmt.Errorf(
			"acknowledge delivered reminder: %w: %s",
			errUnsupportedDeliveredReminderStatus,
			status,
		)
	}

	completedAt := time.Now()
	if err = b.service.CompleteOccurrence(ctx, reminderID, status, &completedAt); err != nil {
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
	case telegramui.FieldTitle:
		draft.title = text
	case telegramui.FieldDescription:
		draft.description = text
	case telegramui.FieldIntrusiveInterval:
		b.mu.Unlock()

		return b.applyIntrusiveInterval(ctx, chatID, text)
	case telegramui.FieldRepeatInterval:
		b.mu.Unlock()

		return b.applyDraftIntervalDays(chatID, text)
	default:
		if draft.screen != telegramui.ScreenTime {
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
	draft.screen = telegramui.ScreenMain
	b.draftByChat[chatID] = draft
	b.mu.Unlock()

	return true
}

func (b *ReminderBot) applyIntrusiveInterval(ctx context.Context, chatID int64, text string) bool {
	minutes, err := strconv.Atoi(text)
	if err != nil {
		b.setDraftNotice(chatID, "Введите интервал в минутах от 1 до 120.")

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
		b.setDraftNotice(chatID, "Введите интервал в минутах от 1 до 120.")

		return true
	}

	b.deleteDraft(chatID)

	return true
}

func (b *ReminderBot) applyDraftIntervalDays(chatID int64, text string) bool {
	days, err := strconv.Atoi(text)
	if err != nil || days <= 0 {
		b.setDraftScreenNotice(chatID, telegramui.ScreenInterval, "Введите интервал в днях: 1, 2, 3 ...")

		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.intervalDays = days
	draft.repeat = telegramui.RepeatCustomInterval

	draft.waitingFor = ""
	draft.notice = ""
	draft.screen = telegramui.ScreenMain
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
		draft.screen = telegramui.ScreenTime
	} else {
		draft.notice = ""
		draft.screen = telegramui.ScreenMain
	}

	draft.waitingFor = ""

	return true
}

func (b *ReminderBot) applyDraftDateCallback(chatID int64, data string) bool {
	if value, ok := strings.CutPrefix(data, telegramui.CallbackMonthPrefix); ok {
		return b.applyDraftMonth(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, telegramui.CallbackDatePrefix); ok {
		return b.applyDraftDate(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, telegramui.CallbackTimeSetPrefix); ok {
		return b.applyDraftTime(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, telegramui.CallbackTimeShiftPrefix); ok {
		return b.applyDraftTimeShift(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, telegramui.CallbackRepeatPrefix); ok {
		return b.applyDraftRepeat(chatID, value)
	}

	if value, ok := strings.CutPrefix(data, telegramui.CallbackWeekdayPrefix); ok {
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
	case telegramui.RepeatNone:
		return b.createDraftOneTimeReminder(ctx, chatID, draft, remindAt)
	case telegramui.RepeatWeekdays:
		return b.createDraftWeekdaysReminder(ctx, chatID, draft, remindAt, database.WeekdaysMaskWorkdays)
	case telegramui.RepeatCustomWeekdays:
		return b.createDraftWeekdaysReminder(ctx, chatID, draft, remindAt, draft.weekdaysMask)
	case telegramui.RepeatCustomDates:
		return b.createDraftCustomDatesReminder(ctx, chatID, draft)
	case telegramui.RepeatCustomInterval:
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

func intervalDaysForRepeat(repeat telegramui.Repeat) (int, bool) {
	switch repeat {
	case telegramui.RepeatEveryDay:
		return 1, true
	case telegramui.RepeatEveryTwoDays:
		return 2, true
	case telegramui.RepeatEveryWeek:
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

	nowMonth := telegramui.FirstDayOfMonth(b.now(chatID))
	if telegramui.FirstDayOfMonth(month).Before(nowMonth) {
		draft.notice = "Нельзя выбрать месяц в прошлом. Выберите текущий или будущий месяц."
		draft.screen = telegramui.ScreenCalendar
		b.draftByChat[chatID] = draft

		return true
	}

	draft.calendarMonth = telegramui.FirstDayOfMonth(month)
	draft.notice = ""
	draft.screen = telegramui.ScreenCalendar
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

	draft.repeat = telegramui.Repeat(value)
	draft.notice = ""

	switch draft.repeat {
	case telegramui.RepeatCustomDates:
		draft.screen = telegramui.ScreenCalendar
	case telegramui.RepeatCustomWeekdays:
		draft.screen = telegramui.ScreenWeekdays
	case telegramui.RepeatCustomInterval:
		if draft.intervalDays == 0 {
			draft.intervalDays = 1
		}

		draft.waitingFor = telegramui.FieldRepeatInterval
		draft.screen = telegramui.ScreenInterval
	default:
		draft.screen = telegramui.ScreenMain
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
	draft.repeat = telegramui.RepeatCustomWeekdays
	draft.notice = ""
	draft.screen = telegramui.ScreenWeekdays
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
	if date.Before(telegramui.BeginningOfDay(now)) {
		draft.notice = "Нельзя выбрать дату в прошлом. Выберите сегодняшнюю или будущую дату."
		draft.screen = telegramui.ScreenCalendar
		b.draftByChat[chatID] = draft

		return true
	}

	if draft.repeat == telegramui.RepeatCustomDates {
		draft.customDates = toggleDraftCustomDate(draft.customDates, date)
		draft.calendarMonth = telegramui.FirstDayOfMonth(date)
		draft.notice = ""
		draft.screen = telegramui.ScreenCalendar
		b.draftByChat[chatID] = draft

		return true
	}

	draft.date = date

	draft.calendarMonth = telegramui.FirstDayOfMonth(date)
	if !draft.remindAt(location).After(now) {
		next := nextReminderTime(now)
		draft.hour = next.Hour()
		draft.minute = next.Minute()
	}

	draft.notice = ""
	draft.screen = telegramui.ScreenTime
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

	draft.screen = telegramui.ScreenMain
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

	if draft.screen == telegramui.ScreenInterval {
		days := draft.intervalDays
		if days == 0 {
			days = 1
		}

		days += minutes / 1440
		if days < 1 {
			days = 1
		}

		draft.intervalDays = days
		draft.repeat = telegramui.RepeatCustomInterval
		draft.notice = ""
		draft.screen = telegramui.ScreenInterval
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

	draft.screen = telegramui.ScreenTime
	b.draftByChat[chatID] = draft

	return true
}

func (b *ReminderBot) setDraftWaitingFor(chatID int64, field telegramui.Field) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.waitingFor = field
	draft.notice = ""
	draft.screen = telegramui.ScreenMain
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftNotice(chatID int64, notice string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.notice = notice
	draft.waitingFor = ""
	draft.screen = telegramui.ScreenMain
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftTimeNotice(chatID int64, notice string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.notice = notice
	draft.waitingFor = ""
	draft.screen = telegramui.ScreenTime
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftScreenNotice(chatID int64, screen telegramui.Screen, notice string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	draft.notice = notice
	draft.waitingFor = ""
	draft.screen = screen
	b.draftByChat[chatID] = draft
}

func (b *ReminderBot) setDraftScreen(chatID int64, screen telegramui.Screen) {
	b.mu.Lock()
	defer b.mu.Unlock()

	draft := b.draftByChat[chatID]
	if draft.calendarMonth.IsZero() {
		draft.calendarMonth = telegramui.FirstDayOfMonth(b.now(chatID))
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

func (b *ReminderBot) setReminderPage(chatID int64, page reminderPage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pageByChat == nil {
		b.pageByChat = make(map[int64]reminderPage)
	}

	b.pageByChat[chatID] = page
}

func (b *ReminderBot) reminderPage(chatID int64) reminderPage {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.pageByChat[chatID]
}

func (b *ReminderBot) setTimezoneIndex(chatID int64, index int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.timezoneByChat == nil {
		b.timezoneByChat = make(map[int64]int)
	}

	b.timezoneByChat[chatID] = index
}

func (b *ReminderBot) timezoneIndex(chatID int64) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.timezoneByChat[chatID]
}

func (b *ReminderBot) setChatMenu(chatID int64, menu chatMenu) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.menuByChat == nil {
		b.menuByChat = make(map[int64]chatMenu)
	}

	b.menuByChat[chatID] = menu
}

func (b *ReminderBot) chatMenu(chatID int64) chatMenu {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.menuByChat[chatID]
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
		screen:        telegramui.ScreenMain,
		calendarMonth: telegramui.FirstDayOfMonth(next),
	}
}

func uiDraft(draft reminderDraft) telegramui.Draft {
	return telegramui.Draft{
		Title:         draft.title,
		Description:   draft.description,
		Notice:        draft.notice,
		Date:          draft.date,
		Hour:          draft.hour,
		Minute:        draft.minute,
		Repeat:        draft.repeat,
		CustomDates:   draft.customDates,
		WeekdaysMask:  draft.weekdaysMask,
		IntervalDays:  draft.intervalDays,
		Screen:        draft.screen,
		CalendarMonth: draft.calendarMonth,
		WaitingFor:    draft.waitingFor,
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
