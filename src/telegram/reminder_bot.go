package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/logger"
	notificationtelegram "github.com/EgorKo25/EasyNote/src/notification/telegram"
	telegrammodel "github.com/EgorKo25/EasyNote/src/telegram/model"
	telegramui "github.com/EgorKo25/EasyNote/src/telegram/ui"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

var ErrUnsupportedReminderRepeat = errors.New("unsupported reminder repeat")
var errUnsupportedDeliveredReminderStatus = errors.New("unsupported delivered reminder status")

const (
	commandStart        = "/start"
	draftPastTimeNotice = "Выбранное время уже прошло. Выберите будущее время."
)

func NewReminderBot(
	sender *notificationtelegram.Sender,
	service ReminderService,
	repository ReminderRepository,
	cfg config.Telegram,
) *ReminderBot {
	return &ReminderBot{
		bot:            sender.Bot(),
		cfg:            cfg,
		service:        service,
		userRepo:       repository,
		log:            logger.Get("telegram/reminder-bot"),
		draftByChat:    make(map[int64]reminderDraft),
		uiByChat:       make(map[int64]int),
		locationByChat: make(map[int64]*time.Location),
		listByChat:     make(map[int64]reminderListState),
		timezoneByChat: make(map[int64]int),
		menuByChat:     make(map[int64]chatMenu),
	}
}

type ReminderBot struct {
	bot      *telego.Bot
	cfg      config.Telegram
	service  ReminderService
	userRepo ReminderRepository
	log      logger.Logger

	mu             sync.Mutex
	locationMu     sync.RWMutex
	draftByChat    map[int64]reminderDraft
	uiByChat       map[int64]int
	locationByChat map[int64]*time.Location
	listByChat     map[int64]reminderListState
	timezoneByChat map[int64]int
	menuByChat     map[int64]chatMenu
}

type reminderListState struct {
	kind   reminderListKind
	offset int
}

type reminderListKind string

const (
	reminderPageSize                      = 2
	reminderListActive   reminderListKind = "active"
	reminderListHistory  reminderListKind = "history"
	reminderListHoliday  reminderListKind = "holiday"
	reminderListShopping reminderListKind = "shopping"
)

type chatMenu string

const (
	chatMenuMain         chatMenu = "main"
	chatMenuCreate       chatMenu = "create"
	chatMenuReminders    chatMenu = "reminders"
	chatMenuReminderList chatMenu = "reminder_list"
	chatMenuHolidays     chatMenu = "holidays"
	chatMenuShopping     chatMenu = "shopping"
	chatMenuSettings     chatMenu = "settings"
	chatMenuTimezone     chatMenu = "timezone"
	chatMenuIntrusive    chatMenu = "intrusive"
)

//go:generate go run go.uber.org/mock/mockgen -source=reminder_bot.go -destination=mocks/reminder_bot_mock.go -package=mocks
type ReminderService interface {
	CreateReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
	) (telegrammodel.ReminderView, error)
	CreateIntervalReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
		intervalDays int,
	) (telegrammodel.ReminderView, error)
	CreateWeekdaysReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		remindAt time.Time,
		weekdaysMask int,
	) (telegrammodel.ReminderView, error)
	CreateCustomDatesReminder(
		ctx context.Context,
		telegramChatID int64,
		title string,
		description string,
		selectedDates []time.Time,
		hour int,
		minute int,
		location *time.Location,
	) (telegrammodel.ReminderView, error)
	ReminderByID(ctx context.Context, telegramChatID int64, reminderID int64) (telegrammodel.ReminderView, error)
	UpdateReminder(
		ctx context.Context,
		telegramChatID int64,
		reminderID int64,
		title string,
		description string,
		remindAt time.Time,
	) (telegrammodel.ReminderView, error)
	DeleteReminder(ctx context.Context, telegramChatID int64, reminderID int64) error
	AcknowledgeReminder(
		ctx context.Context,
		telegramChatID int64,
		reminderID int64,
		status telegrammodel.ReminderStatus,
	) error
	CreateHoliday(ctx context.Context, request telegrammodel.HolidayRequest) (telegrammodel.HolidayView, error)
	UpdateHoliday(ctx context.Context, request telegrammodel.HolidayUpdateRequest) (telegrammodel.HolidayView, error)
	FindHolidaysByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]telegrammodel.HolidayView, error)
	HolidayByID(ctx context.Context, telegramChatID int64, holidayID int64) (telegrammodel.HolidayView, error)
	DeleteHoliday(ctx context.Context, telegramChatID int64, holidayID int64) error
	CreateShoppingList(
		ctx context.Context,
		telegramChatID int64,
		items []telegrammodel.ShoppingListItem,
	) (telegrammodel.ShoppingListView, error)
	FindShoppingListsByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]telegrammodel.ShoppingListView, error)
	ShoppingListByID(ctx context.Context, telegramChatID int64, listID int64) (telegrammodel.ShoppingListView, error)
	ToggleShoppingListItem(
		ctx context.Context,
		telegramChatID int64,
		itemID int64,
	) (telegrammodel.ShoppingListView, error)
}

type ReminderRepository interface {
	UserSettings(ctx context.Context, telegramChatID int64) (telegrammodel.UserSettings, error)
	UpdateUserTimezone(ctx context.Context, telegramChatID int64, timezone string) error
	UpdateIntrusiveSettings(
		ctx context.Context,
		telegramChatID int64,
		enabled bool,
		interval time.Duration,
	) error
	ReminderStatsByChat(ctx context.Context, telegramChatID int64) (telegrammodel.ReminderStats, error)
	FindRemindersByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]telegrammodel.ReminderView, error)
	FindReminderHistoryByChat(
		ctx context.Context,
		telegramChatID int64,
		limit int,
		offset int,
	) ([]telegrammodel.ReminderView, error)
}

type reminderDraft struct {
	id            int64
	kind          telegramui.DraftKind
	title         string
	description   string
	notice        string
	shoppingItems []telegrammodel.ShoppingListItem
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

	defer b.deleteMessage(ctx, chatID, message.MessageID)

	if handled, err := b.handleKnownMessage(ctx, chatID, text); handled || err != nil {
		if err != nil {
			return err
		}

		return nil
	}

	if b.applyDraftText(ctx, chatID, text) {
		if err := b.showDraftTextResult(ctx, chatID); err != nil {
			return err
		}

		return nil
	}

	if err := b.replaceUI(
		ctx,
		chatID,
		"<i>Не понял команду. Используйте основную клавиатуру.</i>",
		telegramui.MainKeyboard(),
	); err != nil {
		return err
	}

	return nil
}

func (b *ReminderBot) showDraftTextResult(ctx context.Context, chatID int64) error {
	if b.draftExists(chatID) {
		return b.editDraft(ctx, chatID, b.currentUIMessageID(chatID))
	}

	if b.chatMenu(chatID) == chatMenuIntrusive {
		return b.editIntrusiveSettings(ctx, chatID, b.currentUIMessageID(chatID))
	}

	return b.sendSettings(ctx, chatID)
}

func (b *ReminderBot) handleKnownMessage(
	ctx context.Context,
	chatID int64,
	text string,
) (bool, error) {
	err, handled := b.runKnownMessageCommand(ctx, chatID, text)
	if !handled {
		return false, nil
	}

	if err != nil {
		return true, err
	}

	return true, nil
}

func (b *ReminderBot) runKnownMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	if handled, err := b.handleSelectedReplyMessage(ctx, chatID, text); handled || err != nil {
		return err, handled
	}

	if err, handled := b.runMainMessageCommand(ctx, chatID, text); handled {
		return err, true
	}

	if err, handled := b.runDraftMessageCommand(ctx, chatID, text); handled {
		return err, true
	}

	if err, handled := b.runSettingsMessageCommand(ctx, chatID, text); handled {
		return err, true
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
	case commandStart, telegramui.ButtonStart:
		return b.sendStart(ctx, chatID), true
	case telegramui.ButtonCreateReminder, "➕ " + telegramui.ButtonCreateReminder:
		return b.sendCreate(ctx, chatID), true
	case telegramui.ButtonReminders, "⏰ " + telegramui.ButtonReminders:
		return b.sendReminders(ctx, chatID), true
	case telegramui.ButtonShoppingLists:
		return b.sendShoppingListPage(ctx, chatID, 0), true
	case telegramui.ButtonHolidays:
		return b.sendHolidayListPage(ctx, chatID, 0), true
	case telegramui.ButtonSettings, "⚙️ " + telegramui.ButtonSettings:
		return b.sendSettings(ctx, chatID), true
	case telegramui.ButtonBack:
		return b.handleBackMessage(ctx, chatID), true
	default:
		return b.runCreateKindMessageCommand(ctx, chatID, text)
	}
}

func (b *ReminderBot) runCreateKindMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case telegramui.ButtonCreateReminderKind:
		return b.startReminderDraft(ctx, chatID), true
	case telegramui.ButtonCreateHolidayKind:
		return b.startHolidayDraft(ctx, chatID), true
	case telegramui.ButtonCreateShoppingKind:
		return b.startShoppingDraft(ctx, chatID), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runDraftMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	if !b.draftExists(chatID) {
		return nil, false
	}

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
	case telegramui.ButtonDraftTitle,
		telegramui.ButtonDraftEnterTitle,
		telegramui.ButtonDraftEditTitle,
		telegramui.ButtonHolidayName,
		telegramui.ButtonHolidayEnterName,
		telegramui.ButtonHolidayEditName:
		b.setDraftWaitingFor(chatID, telegramui.FieldTitle)

		return b.editDraftText(ctx, chatID), true
	case telegramui.ButtonDraftDescription,
		telegramui.ButtonDraftEnterDescription,
		telegramui.ButtonDraftEditDescription,
		telegramui.ButtonHolidayPerson,
		telegramui.ButtonHolidayEnterPerson,
		telegramui.ButtonHolidayEditPerson:
		b.setDraftWaitingFor(chatID, telegramui.FieldDescription)

		return b.editDraftText(ctx, chatID), true
	case telegramui.ButtonShoppingAddItem:
		b.setDraftWaitingFor(chatID, telegramui.FieldShoppingItem)

		return b.editDraftText(ctx, chatID), true
	case telegramui.ButtonDraftDate, telegramui.ButtonDraftEditDateTime, "Изменить дату":
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
	case telegramui.ButtonRepeatNone:
		b.applyDraftRepeat(chatID, string(telegramui.RepeatNone))

		return b.replaceDraft(ctx, chatID), true
	default:
		return nil, false
	}
}

func (b *ReminderBot) runDraftTimeMessageCommand(ctx context.Context, chatID int64, text string) (error, bool) {
	switch text {
	case telegramui.ButtonDone:
		return b.handleDoneMessage(ctx, chatID), true
	default:
		minutes, ok := replyShiftMinutes(text)
		if !ok {
			return nil, false
		}

		return b.handleTimeShiftMessage(ctx, chatID, minutes), true
	}
}

func replyShiftMinutes(text string) (int, bool) {
	switch text {
	case telegramui.ButtonMinusHour:
		return -60, true
	case telegramui.ButtonPlusHour:
		return 60, true
	case telegramui.ButtonMinusFiveMinutes:
		return -5, true
	case telegramui.ButtonPlusFiveMinutes:
		return 5, true
	case telegramui.ButtonMinusMinute:
		return -1, true
	case telegramui.ButtonPlusMinute:
		return 1, true
	case telegramui.ButtonMinusFifteenMinutes:
		return -15, true
	case telegramui.ButtonPlusFifteenMinutes:
		return 15, true
	default:
		return 0, false
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
		return b.replaceUI(
			ctx,
			chatID,
			"<b>Язык</b>\n\n<i>Язык приложения: русский.</i>",
			telegramui.SettingsKeyboard(),
		), true
	case telegramui.ButtonSettingsTimezone:
		return b.sendTimezoneSettings(ctx, chatID), true
	case telegramui.ButtonSettingsIntrusive:
		return b.sendIntrusiveSettings(ctx, chatID), true
	case telegramui.ButtonIntrusiveEnabled, telegramui.ButtonIntrusiveDisabled:
		return b.toggleIntrusiveSettings(ctx, chatID), true
	case telegramui.ButtonSettingsSave:
		return b.saveSettingsMenu(ctx, chatID), true
	default:
		return nil, false
	}
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
		return b.sendStart(ctx, chatID)
	}

	if handled, err := b.handleSettingsCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if handled, err := b.handleDraftCommandCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if handled, err := b.handleDraftValueCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if handled, err := b.handleHolidayCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	if handled, err := b.handleShoppingCallback(ctx, chatID, messageID, query.Data); handled {
		return err
	}

	return b.handleReminderCallback(ctx, chatID, messageID, query.Data)
}

func (b *ReminderBot) handleDraftValueCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	if !b.applyDraftDateCallback(chatID, data) {
		return false, nil
	}

	if b.draft(chatID).screen != telegramui.ScreenCalendar {
		return true, b.replaceDraft(ctx, chatID)
	}

	return true, b.editDraft(ctx, chatID, messageID)
}

func (b *ReminderBot) handleHolidayCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch {
	case data == telegramui.CallbackHolidayBackList:
		return true, b.sendHolidayListPage(ctx, chatID, b.reminderListState(chatID).offset)
	case strings.HasPrefix(data, telegramui.CallbackHolidayEditPrefix):
		holidayID, err := strconv.ParseInt(strings.TrimPrefix(data, telegramui.CallbackHolidayEditPrefix), 10, 64)
		if err != nil {
			return true, fmt.Errorf("parse holiday id: %w", err)
		}

		return true, b.startHolidayDraftForEdit(ctx, chatID, messageID, holidayID)
	case strings.HasPrefix(data, telegramui.CallbackHolidayDeletePrefix):
		holidayID, err := strconv.ParseInt(strings.TrimPrefix(data, telegramui.CallbackHolidayDeletePrefix), 10, 64)
		if err != nil {
			return true, fmt.Errorf("parse holiday id: %w", err)
		}

		return true, b.deleteHoliday(ctx, chatID, holidayID)
	default:
		return false, nil
	}
}

func (b *ReminderBot) handleShoppingCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch {
	case data == telegramui.CallbackShoppingBackList:
		return true, b.sendShoppingListPage(ctx, chatID, b.reminderListState(chatID).offset)
	case strings.HasPrefix(data, telegramui.CallbackShoppingItemPrefix):
		itemID, err := strconv.ParseInt(strings.TrimPrefix(data, telegramui.CallbackShoppingItemPrefix), 10, 64)
		if err != nil {
			return true, fmt.Errorf("parse shopping item id: %w", err)
		}

		return true, b.toggleShoppingListItem(ctx, chatID, messageID, itemID)
	default:
		return false, nil
	}
}

func (b *ReminderBot) handleReminderCallback(ctx context.Context, chatID int64, messageID int, data string) error {
	if handled, err := b.handleReminderActionCallback(ctx, chatID, messageID, data); handled {
		return err
	}

	switch data {
	case telegramui.CallbackCreateReminder:
		return b.startReminderDraftInMessage(ctx, chatID, messageID)
	case telegramui.CallbackReminderList, telegramui.CallbackReminderBackList:
		return b.editReminderList(ctx, chatID, messageID)
	case telegramui.CallbackReminderHistory:
		return b.editReminderHistory(ctx, chatID, messageID)
	default:
		return nil
	}
}

func (b *ReminderBot) handleReminderActionCallback(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
) (bool, error) {
	switch {
	case strings.HasPrefix(data, telegramui.CallbackReminderDone):
		return true, b.acknowledgeReminder(
			ctx,
			chatID,
			messageID,
			data,
			telegramui.CallbackReminderDone,
			telegrammodel.ReminderStatusCompleted,
		)
	case strings.HasPrefix(data, telegramui.CallbackReminderCancelDelivered):
		return true, b.acknowledgeReminder(
			ctx,
			chatID,
			messageID,
			data,
			telegramui.CallbackReminderCancelDelivered,
			telegrammodel.ReminderStatusCanceled,
		)
	case strings.HasPrefix(data, telegramui.CallbackReminderEditPrefix):
		reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, telegramui.CallbackReminderEditPrefix), 10, 64)
		if err != nil {
			return true, fmt.Errorf("parse reminder id: %w", err)
		}

		return true, b.startReminderDraftForEdit(ctx, chatID, messageID, reminderID)
	case strings.HasPrefix(data, telegramui.CallbackDeletePrefix):
		return true, b.deleteReminder(ctx, chatID, messageID, data)
	case strings.HasPrefix(data, telegramui.CallbackViewPrefix):
		return true, b.viewReminder(ctx, chatID, messageID, data)
	default:
		return false, nil
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
		return true, b.replaceUI(
			ctx,
			chatID,
			"<b>Язык</b>\n\n<i>Язык приложения: русский.</i>",
			telegramui.SettingsKeyboard(),
		)
	case telegramui.CallbackSettingsIntrusive:
		b.setDraftWaitingFor(chatID, telegramui.FieldIntrusiveInterval)

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	case telegramui.CallbackSettingsIntrusiveOn:
		if err := b.setIntrusiveEnabled(ctx, chatID, true); err != nil {
			return true, fmt.Errorf("enable intrusive reminders: %w", err)
		}

		return true, b.editIntrusiveSettings(ctx, chatID, messageID)
	case telegramui.CallbackSettingsIntrusiveOff:
		if err := b.setIntrusiveEnabled(ctx, chatID, false); err != nil {
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
	case telegramui.CallbackShoppingAddItem:
		b.setDraftWaitingFor(chatID, telegramui.FieldShoppingItem)

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

		return true, b.sendStart(ctx, chatID)
	default:
		return false, nil
	}
}

func (b *ReminderBot) sendStart(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuMain)

	stats, err := b.reminderStats(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load reminder stats: %w", err)
	}

	return b.replaceUI(ctx, chatID, telegramui.MainMenuText(stats), telegramui.MainKeyboard())
}

func (b *ReminderBot) reminderStats(ctx context.Context, chatID int64) (telegrammodel.ReminderStats, error) {
	stats, err := b.userRepo.ReminderStatsByChat(ctx, chatID)
	if err != nil {
		return telegrammodel.ReminderStats{}, fmt.Errorf("get reminder stats by chat: %w", err)
	}

	return stats, nil
}

func (b *ReminderBot) userSettings(ctx context.Context, chatID int64) (telegrammodel.UserSettings, error) {
	settings, err := b.userRepo.UserSettings(ctx, chatID)
	if err != nil {
		return telegrammodel.UserSettings{}, fmt.Errorf("get user settings: %w", err)
	}

	return settings, nil
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
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	b.mu.Lock()
	b.draftByChat[chatID] = newReminderDraft(b.now(chatID))
	b.mu.Unlock()

	return b.replaceDraft(ctx, chatID)
}

func (b *ReminderBot) startReminderDraftInMessage(ctx context.Context, chatID int64, messageID int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	b.mu.Lock()
	b.draftByChat[chatID] = newReminderDraft(b.now(chatID))
	b.mu.Unlock()

	return b.replaceDraft(ctx, chatID)
}

func (b *ReminderBot) startHolidayDraft(ctx context.Context, chatID int64) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	draft := newReminderDraft(b.now(chatID))
	draft.kind = telegramui.DraftKindHoliday

	b.mu.Lock()
	b.draftByChat[chatID] = draft
	b.mu.Unlock()

	return b.replaceDraft(ctx, chatID)
}

func (b *ReminderBot) startShoppingDraft(ctx context.Context, chatID int64) error {
	draft := reminderDraft{kind: telegramui.DraftKindShopping}

	b.mu.Lock()
	b.draftByChat[chatID] = draft
	b.mu.Unlock()

	return b.replaceDraft(ctx, chatID)
}

func (b *ReminderBot) startHolidayDraftForEdit(
	ctx context.Context,
	chatID int64,
	messageID int,
	holidayID int64,
) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	holiday, err := b.service.HolidayByID(ctx, chatID, holidayID)
	if err != nil {
		return fmt.Errorf("load holiday for edit: %w", err)
	}

	remindAt := holiday.RemindAt.In(b.location(chatID))
	draft := reminderDraft{
		id:            holiday.ID,
		kind:          telegramui.DraftKindHoliday,
		title:         holiday.HolidayName,
		description:   holiday.RecipientName,
		date:          remindAt,
		hour:          remindAt.Hour(),
		minute:        remindAt.Minute(),
		screen:        telegramui.ScreenMain,
		calendarMonth: telegramui.FirstDayOfMonth(remindAt),
	}

	b.mu.Lock()
	b.draftByChat[chatID] = draft
	b.mu.Unlock()

	return b.editDraft(ctx, chatID, messageID)
}

func (b *ReminderBot) startReminderDraftForEdit(
	ctx context.Context,
	chatID int64,
	messageID int,
	reminderID int64,
) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	reminder, err := b.service.ReminderByID(ctx, chatID, reminderID)
	if err != nil {
		return fmt.Errorf("load reminder for edit: %w", err)
	}

	remindAt := reminder.RemindAt.In(b.location(chatID))
	draft := newReminderDraft(remindAt)
	draft.id = reminder.ID
	draft.title = reminder.Title
	draft.description = reminder.Description
	draft.date = remindAt
	draft.hour = remindAt.Hour()
	draft.minute = remindAt.Minute()
	draft.calendarMonth = telegramui.FirstDayOfMonth(remindAt)

	b.mu.Lock()
	b.draftByChat[chatID] = draft
	b.mu.Unlock()

	return b.editDraft(ctx, chatID, messageID)
}

func (b *ReminderBot) replaceDraft(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuCreate)

	draft := b.draft(chatID)
	uiDraftValue := uiDraft(draft)

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.DraftText(uiDraftValue, b.location(chatID)),
		telegramui.DraftKeyboard(uiDraftValue, b.now(chatID)),
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

func (b *ReminderBot) sendHolidayListPage(ctx context.Context, chatID int64, offset int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	offset = normalizePageOffset(offset)

	holidays, err := b.service.FindHolidaysByChat(ctx, chatID, reminderPageSize, offset)
	if err != nil {
		return fmt.Errorf("load holidays: %w", err)
	}

	if len(holidays) == 0 && offset > 0 {
		return b.sendHolidayListPage(ctx, chatID, offset-reminderPageSize)
	}

	b.setReminderListState(chatID, reminderListState{kind: reminderListHoliday, offset: offset})
	b.setChatMenu(chatID, chatMenuHolidays)

	return b.replaceUI(ctx, chatID, telegramui.HolidaysText(holidays), telegramui.HolidayListKeyboard(holidays))
}

func (b *ReminderBot) sendShoppingListPage(ctx context.Context, chatID int64, offset int) error {
	offset = normalizePageOffset(offset)

	lists, err := b.service.FindShoppingListsByChat(ctx, chatID, reminderPageSize, offset)
	if err != nil {
		return fmt.Errorf("load shopping lists: %w", err)
	}

	if len(lists) == 0 && offset > 0 {
		return b.sendShoppingListPage(ctx, chatID, offset-reminderPageSize)
	}

	b.setReminderListState(chatID, reminderListState{kind: reminderListShopping, offset: offset})
	b.setChatMenu(chatID, chatMenuShopping)

	return b.replaceUI(ctx, chatID, telegramui.ShoppingListsText(lists), telegramui.ShoppingListKeyboard(lists))
}

func (b *ReminderBot) editDraftText(ctx context.Context, chatID int64) error {
	return b.editUI(
		ctx,
		chatID,
		b.currentUIMessageID(chatID),
		telegramui.DraftText(uiDraft(b.draft(chatID)), b.location(chatID)),
		nil,
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

	reminders, err := b.userRepo.FindReminderHistoryByChat(ctx, chatID, reminderPageSize, 0)
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
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	offset = normalizePageOffset(offset)

	reminders, err := b.userRepo.FindRemindersByChat(ctx, chatID, reminderPageSize, offset)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	if len(reminders) == 0 && offset > 0 {
		return b.sendReminderListPage(ctx, chatID, offset-reminderPageSize)
	}

	b.setReminderListState(chatID, reminderListState{kind: reminderListActive, offset: offset})
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

	reminders, err := b.userRepo.FindReminderHistoryByChat(ctx, chatID, reminderPageSize, offset)
	if err != nil {
		return fmt.Errorf("load reminder history: %w", err)
	}

	if len(reminders) == 0 && offset > 0 {
		return b.sendReminderHistoryPage(ctx, chatID, offset-reminderPageSize)
	}

	b.setReminderListState(chatID, reminderListState{kind: reminderListHistory, offset: offset})
	b.setChatMenu(chatID, chatMenuReminderList)

	return b.replaceUI(
		ctx,
		chatID,
		telegramui.ReminderHistoryText(reminders),
		telegramui.ReminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) sendShiftedReminderPage(ctx context.Context, chatID int64, shift int) error {
	list := b.reminderListState(chatID)
	switch list.kind {
	case reminderListHistory:
		return b.sendReminderHistoryPage(ctx, chatID, list.offset+shift)
	case reminderListHoliday:
		return b.sendHolidayListPage(ctx, chatID, list.offset+shift)
	case reminderListShopping:
		return b.sendShoppingListPage(ctx, chatID, list.offset+shift)
	default:
		return b.sendReminderListPage(ctx, chatID, list.offset+shift)
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

	settings, err := b.userSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.replaceUI(ctx, chatID, telegramui.SettingsText(settings), telegramui.SettingsKeyboard())
}

func (b *ReminderBot) editSettings(ctx context.Context, chatID int64, messageID int) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.userSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.replaceUI(ctx, chatID, telegramui.SettingsText(settings), telegramui.SettingsKeyboard())
}

func (b *ReminderBot) editIntrusiveSettings(ctx context.Context, chatID int64, messageID int) error {
	b.setChatMenu(chatID, chatMenuIntrusive)
	b.setDraftWaitingFor(chatID, telegramui.FieldIntrusiveInterval)

	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.userSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	return b.editUI(
		ctx,
		chatID,
		messageID,
		telegramui.IntrusiveSettingsText(settings),
		telegramui.IntrusiveSettingsKeyboard(settings.IntrusiveEnabled),
	)
}

func (b *ReminderBot) sendIntrusiveSettings(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuIntrusive)
	b.setDraftWaitingFor(chatID, telegramui.FieldIntrusiveInterval)

	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	settings, err := b.userSettings(ctx, chatID)
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
	settings, err := b.userSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	if err = b.userRepo.UpdateIntrusiveSettings(
		ctx,
		chatID,
		!settings.IntrusiveEnabled,
		settings.IntrusiveInterval,
	); err != nil {
		return fmt.Errorf("toggle intrusive reminders: %w", err)
	}

	return b.sendIntrusiveSettings(ctx, chatID)
}

func (b *ReminderBot) setIntrusiveEnabled(ctx context.Context, chatID int64, enabled bool) error {
	settings, err := b.userSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	interval := settings.IntrusiveInterval
	if interval == 0 {
		interval = 30 * time.Minute
	}

	if err = b.userRepo.UpdateIntrusiveSettings(ctx, chatID, enabled, interval); err != nil {
		return fmt.Errorf("update intrusive settings: %w", err)
	}

	return nil
}

func (b *ReminderBot) editTimezoneSettings(ctx context.Context, chatID int64, messageID int, data string) error {
	var index int

	if data == telegramui.CallbackSettingsTimezone {
		settings, err := b.userSettings(ctx, chatID)
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
		"<b>Часовой пояс</b>\n\n<i>Выберите часовой пояс.</i>",
		telegramui.TimezoneKeyboard(index, b.now(chatID)),
	)
}

func (b *ReminderBot) sendTimezoneSettings(ctx context.Context, chatID int64) error {
	b.setChatMenu(chatID, chatMenuTimezone)

	settings, err := b.userSettings(ctx, chatID)
	if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	index := telegramui.TimezoneIndex(settings.Timezone)
	b.setTimezoneIndex(chatID, index)

	return b.replaceUI(
		ctx,
		chatID,
		"<b>Часовой пояс</b>\n\n<i>Выберите часовой пояс.</i>",
		telegramui.TimezoneKeyboard(index, b.now(chatID)),
	)
}

func (b *ReminderBot) shiftTimezone(ctx context.Context, chatID int64, shift int) error {
	index := telegramui.TimezoneIndexByShift(b.timezoneIndex(chatID), shift)
	b.setTimezoneIndex(chatID, index)

	return b.replaceUI(
		ctx,
		chatID,
		"<b>Часовой пояс</b>\n\n<i>Выберите часовой пояс.</i>",
		telegramui.TimezoneKeyboard(index, b.now(chatID)),
	)
}

func (b *ReminderBot) saveCurrentTimezone(ctx context.Context, chatID int64) error {
	option := telegramui.TimezoneByIndex(b.timezoneIndex(chatID))
	if err := b.userRepo.UpdateUserTimezone(ctx, chatID, option.Timezone); err != nil {
		return fmt.Errorf("set user timezone: %w", err)
	}

	return b.sendSettings(ctx, chatID)
}

func (b *ReminderBot) saveSettingsMenu(ctx context.Context, chatID int64) error {
	if b.chatMenu(chatID) == chatMenuTimezone {
		option := telegramui.TimezoneByIndex(b.timezoneIndex(chatID))
		if err := b.userRepo.UpdateUserTimezone(ctx, chatID, option.Timezone); err != nil {
			return fmt.Errorf("set user timezone: %w", err)
		}
	}

	return b.sendStart(ctx, chatID)
}

func (b *ReminderBot) setTimezoneByCallback(ctx context.Context, chatID int64, messageID int, data string) error {
	index, err := timezoneCallbackIndex(data)
	if err != nil {
		return err
	}

	option := telegramui.TimezoneByIndex(index)
	if err = b.userRepo.UpdateUserTimezone(ctx, chatID, option.Timezone); err != nil {
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
	case chatMenuIntrusive:
		return b.sendSettings(ctx, chatID)
	case chatMenuReminderList:
		return b.sendReminders(ctx, chatID)
	case chatMenuHolidays, chatMenuShopping:
		return b.sendStart(ctx, chatID)
	default:
		return b.sendStart(ctx, chatID)
	}
}

func (b *ReminderBot) handleDoneMessage(ctx context.Context, chatID int64) error {
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

	if handled, err := b.handleListReplyMessage(ctx, chatID, text); handled || err != nil {
		return handled, err
	}

	if b.applyDraftTimeTextByMessage(chatID, text) {
		return true, b.replaceDraft(ctx, chatID)
	}

	return false, nil
}

func (b *ReminderBot) handleListReplyMessage(ctx context.Context, chatID int64, text string) (bool, error) {
	switch b.chatMenu(chatID) {
	case chatMenuReminderList:
		if reminderListCommandText(text) {
			return false, nil
		}

		return true, b.viewReminderByReply(ctx, chatID, text)
	case chatMenuHolidays:
		if reminderListCommandText(text) {
			return false, nil
		}

		return true, b.viewHolidayByReply(ctx, chatID, text)
	case chatMenuShopping:
		if reminderListCommandText(text) {
			return false, nil
		}

		return true, b.viewShoppingListByReply(ctx, chatID, text)
	default:
		return false, nil
	}
}

func reminderListCommandText(text string) bool {
	switch text {
	case telegramui.ButtonBack,
		telegramui.ButtonPreviousPage,
		telegramui.ButtonNextPage,
		telegramui.ButtonReminderList,
		telegramui.ButtonReminderHistory:
		return true
	default:
		return false
	}
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
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	list := b.reminderListState(chatID)

	reminders, err := b.loadReminderPage(ctx, chatID, list)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}

	for _, reminder := range reminders {
		if telegramui.ReminderButtonText(reminder) == text {
			return b.replaceUI(
				ctx,
				chatID,
				telegramui.ReminderViewText(reminder, b.location(chatID)),
				telegramui.ReminderDetailsKeyboard(reminder),
			)
		}
	}

	return b.sendShiftedReminderPage(ctx, chatID, 0)
}

func (b *ReminderBot) viewHolidayByReply(ctx context.Context, chatID int64, text string) error {
	if err := b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	list := b.reminderListState(chatID)

	holidays, err := b.service.FindHolidaysByChat(ctx, chatID, reminderPageSize, list.offset)
	if err != nil {
		return fmt.Errorf("load holidays: %w", err)
	}

	for _, holiday := range holidays {
		if telegramui.HolidayButtonText(holiday) == text {
			return b.replaceUI(
				ctx,
				chatID,
				fmt.Sprintf(
					"Поздравьте <b>%s</b> с <b>%s</b>.\n\n<b>Дата и время:</b> %s",
					html.EscapeString(holiday.RecipientName),
					html.EscapeString(holiday.HolidayName),
					holiday.RemindAt.In(b.location(chatID)).Format("02.01.2006 15:04"),
				),
				telegramui.HolidayDetailsKeyboard(holiday),
			)
		}
	}

	return b.sendHolidayListPage(ctx, chatID, list.offset)
}

func (b *ReminderBot) viewShoppingListByReply(ctx context.Context, chatID int64, text string) error {
	listState := b.reminderListState(chatID)

	lists, err := b.service.FindShoppingListsByChat(ctx, chatID, reminderPageSize, listState.offset)
	if err != nil {
		return fmt.Errorf("load shopping lists: %w", err)
	}

	for _, list := range lists {
		if telegramui.ShoppingListButtonText(list) == text {
			selectedList, loadErr := b.service.ShoppingListByID(ctx, chatID, list.ID)
			if loadErr != nil {
				return fmt.Errorf("load shopping list: %w", loadErr)
			}

			return b.replaceUI(
				ctx,
				chatID,
				telegramui.ShoppingListViewText(selectedList),
				telegramui.ShoppingListDetailsKeyboard(selectedList),
			)
		}
	}

	return b.sendShoppingListPage(ctx, chatID, listState.offset)
}

func (b *ReminderBot) loadReminderPage(
	ctx context.Context,
	chatID int64,
	list reminderListState,
) ([]telegrammodel.ReminderView, error) {
	if list.kind == reminderListHistory {
		reminders, err := b.userRepo.FindReminderHistoryByChat(ctx, chatID, reminderPageSize, list.offset)
		if err != nil {
			return nil, fmt.Errorf("load reminder history page: %w", err)
		}

		return reminders, nil
	}

	reminders, err := b.userRepo.FindRemindersByChat(ctx, chatID, reminderPageSize, list.offset)
	if err != nil {
		return nil, fmt.Errorf("load active reminder page: %w", err)
	}

	return reminders, nil
}

func (b *ReminderBot) saveDraft(ctx context.Context, chatID int64, messageID int) error {
	draft := b.draft(chatID)
	switch draft.kind {
	case telegramui.DraftKindHoliday:
		return b.saveHolidayDraft(ctx, chatID, messageID, draft)
	case telegramui.DraftKindShopping:
		return b.saveShoppingDraft(ctx, chatID, messageID, draft)
	default:
		return b.saveReminderDraft(ctx, chatID, messageID, draft)
	}
}

func (b *ReminderBot) saveReminderDraft(
	ctx context.Context,
	chatID int64,
	messageID int,
	draft reminderDraft,
) error {
	remindAt := b.draftReminderTime(chatID, draft)

	if b.setDraftValidationNotice(chatID, draft, remindAt) {
		return b.replaceDraft(ctx, chatID)
	}

	var (
		reminder telegrammodel.ReminderView
		err      error
	)

	if draft.id == 0 {
		reminder, err = b.createDraftReminder(ctx, chatID, draft, remindAt)
	} else {
		reminder, err = b.service.UpdateReminder(ctx, chatID, draft.id, draft.title, draft.description, remindAt)
	}

	if err != nil {
		return fmt.Errorf("save reminder: %w", err)
	}

	b.deleteDraft(chatID)

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"<i>Напоминание сохранено.</i>\n\n"+telegramui.ReminderViewText(reminder, b.location(chatID)),
		telegramui.ReminderCreatedKeyboard(),
	)
}

func (b *ReminderBot) saveHolidayDraft(
	ctx context.Context,
	chatID int64,
	messageID int,
	draft reminderDraft,
) error {
	remindAt := draft.remindAt(b.location(chatID))

	if b.setHolidayDraftValidationNotice(chatID, draft, remindAt) {
		return b.replaceDraft(ctx, chatID)
	}

	var (
		holiday telegrammodel.HolidayView
		err     error
	)

	request := telegrammodel.HolidayRequest{
		TelegramChatID: chatID,
		HolidayName:    draft.title,
		RecipientName:  draft.description,
		HolidayDate:    draft.date,
		RemindAt:       remindAt,
	}

	if draft.id == 0 {
		holiday, err = b.service.CreateHoliday(ctx, request)
	} else {
		holiday, err = b.service.UpdateHoliday(ctx, telegrammodel.HolidayUpdateRequest{
			ID:             draft.id,
			HolidayRequest: request,
		})
	}

	if err != nil {
		return fmt.Errorf("save holiday: %w", err)
	}

	b.deleteDraft(chatID)

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"<i>Праздник сохранен.</i>\n\n"+fmt.Sprintf(
			"Поздравьте <b>%s</b> с <b>%s</b>.\n\n<b>Дата и время:</b> %s",
			html.EscapeString(holiday.RecipientName),
			html.EscapeString(holiday.HolidayName),
			holiday.RemindAt.In(b.location(chatID)).Format("02.01.2006 15:04"),
		),
		telegramui.HolidayDetailsKeyboard(holiday),
	)
}

func (b *ReminderBot) saveShoppingDraft(
	ctx context.Context,
	chatID int64,
	messageID int,
	draft reminderDraft,
) error {
	if len(draft.shoppingItems) == 0 {
		b.setDraftNotice(chatID, "Добавьте хотя бы один пункт списка.")

		return b.replaceDraft(ctx, chatID)
	}

	list, err := b.service.CreateShoppingList(ctx, chatID, draft.shoppingItems)
	if err != nil {
		return fmt.Errorf("create shopping list: %w", err)
	}

	b.deleteDraft(chatID)

	return b.editUI(
		ctx,
		chatID,
		messageID,
		"<i>Список покупок сохранен.</i>\n\n"+telegramui.ShoppingListViewText(list),
		telegramui.ShoppingListDetailsKeyboard(list),
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

func (b *ReminderBot) setHolidayDraftValidationNotice(chatID int64, draft reminderDraft, remindAt time.Time) bool {
	if draft.title == "" || draft.description == "" || remindAt.IsZero() {
		b.setDraftNotice(chatID, "Чтобы сохранить праздник, добавьте название, имя, дату и время.")

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

	if err = b.loadChatLocation(ctx, chatID); err != nil {
		return err
	}

	list := b.reminderListState(chatID)

	reminders, err := b.userRepo.FindRemindersByChat(ctx, chatID, reminderPageSize, list.offset)
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
		"<i>Напоминание не найдено.</i>\n\n"+telegramui.ReminderListText(reminders),
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

	list := b.reminderListState(chatID)

	reminders, err := b.userRepo.FindRemindersByChat(ctx, chatID, reminderPageSize, list.offset)
	if err != nil {
		return fmt.Errorf("load reminders after delete: %w", err)
	}

	return b.replaceUI(
		ctx,
		chatID,
		"<i>Напоминание удалено.</i>\n\n\n"+telegramui.ReminderListText(reminders),
		telegramui.ReminderListKeyboard(reminders),
	)
}

func (b *ReminderBot) deleteHoliday(ctx context.Context, chatID int64, holidayID int64) error {
	if err := b.service.DeleteHoliday(ctx, chatID, holidayID); err != nil {
		return fmt.Errorf("delete holiday: %w", err)
	}

	return b.sendHolidayListPage(ctx, chatID, b.reminderListState(chatID).offset)
}

func (b *ReminderBot) toggleShoppingListItem(
	ctx context.Context,
	chatID int64,
	messageID int,
	itemID int64,
) error {
	list, err := b.service.ToggleShoppingListItem(ctx, chatID, itemID)
	if err != nil {
		return fmt.Errorf("toggle shopping list item: %w", err)
	}

	return b.editUI(
		ctx,
		chatID,
		messageID,
		telegramui.ShoppingListViewText(list),
		telegramui.ShoppingListDetailsKeyboard(list),
	)
}

func (b *ReminderBot) acknowledgeReminder(
	ctx context.Context,
	chatID int64,
	messageID int,
	data string,
	prefix string,
	status telegrammodel.ReminderStatus,
) error {
	reminderID, err := strconv.ParseInt(strings.TrimPrefix(data, prefix), 10, 64)
	if err != nil {
		return fmt.Errorf("parse delivered reminder id: %w", err)
	}

	if status != telegrammodel.ReminderStatusCompleted && status != telegrammodel.ReminderStatusCanceled {
		return fmt.Errorf(
			"acknowledge delivered reminder: %w: %s",
			errUnsupportedDeliveredReminderStatus,
			status,
		)
	}

	if err = b.service.AcknowledgeReminder(ctx, chatID, reminderID, status); err != nil {
		return fmt.Errorf("acknowledge delivered reminder: %w", err)
	}

	b.deleteMessage(ctx, chatID, messageID)

	return nil
}

func (b *ReminderBot) replaceUI(ctx context.Context, chatID int64, text string, replyMarkup telego.ReplyMarkup) error {
	b.deleteCurrentUI(ctx, chatID)

	params := tu.Message(tu.ID(chatID), text)
	if replyMarkup != nil {
		params.WithReplyMarkup(replyMarkup)
	}

	if hasTelegramHTML(text) {
		params.WithParseMode(telego.ModeHTML)
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
	if hasTelegramHTML(text) {
		params.WithParseMode(telego.ModeHTML)
	}

	message, err := b.bot.EditMessageText(ctx, params)
	if err != nil {
		return fmt.Errorf("edit telegram message: %w", err)
	}

	if message != nil {
		b.setCurrentUIMessageID(chatID, message.MessageID)
	}

	return nil
}

func hasTelegramHTML(text string) bool {
	return strings.Contains(text, "<b>") || strings.Contains(text, "<i>") || strings.Contains(text, "<s>")
}

func (b *ReminderBot) applyDraftText(ctx context.Context, chatID int64, text string) bool {
	b.mu.Lock()

	draft, exists := b.draftByChat[chatID]
	if !exists {
		b.mu.Unlock()

		return false
	}

	if draft.waitingFor == telegramui.FieldIntrusiveInterval {
		b.mu.Unlock()

		return b.applyIntrusiveInterval(ctx, chatID, text)
	}

	switch draft.waitingFor {
	case telegramui.FieldTitle:
		draft.title = text
	case telegramui.FieldDescription:
		draft.description = text
	case telegramui.FieldShoppingItem:
		draft.shoppingItems = append(draft.shoppingItems, telegrammodel.ShoppingListItem{
			Text:     strings.TrimSpace(text),
			Position: len(draft.shoppingItems) + 1,
		})
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
	interval, ok := parseIntrusiveInterval(text)
	if !ok {
		b.deleteDraft(chatID)

		return true
	}

	settings, err := b.userSettings(ctx, chatID)
	if err != nil {
		b.deleteDraft(chatID)

		return true
	}

	if err = b.userRepo.UpdateIntrusiveSettings(
		ctx,
		chatID,
		settings.IntrusiveEnabled,
		interval,
	); err != nil {
		b.deleteDraft(chatID)

		return true
	}

	b.deleteDraft(chatID)

	return true
}

func parseIntrusiveInterval(text string) (time.Duration, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}

	if minutes, err := strconv.Atoi(text); err == nil {
		return validateIntrusiveInterval(time.Duration(minutes) * time.Minute)
	}

	interval, err := time.ParseDuration(text)
	if err != nil {
		return 0, false
	}

	return validateIntrusiveInterval(interval)
}

func validateIntrusiveInterval(interval time.Duration) (time.Duration, bool) {
	if interval < time.Minute || interval > 2*time.Hour || interval%time.Minute != 0 {
		return 0, false
	}

	return interval, true
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
) (telegrammodel.ReminderView, error) {
	switch draft.repeat {
	case telegramui.RepeatNone:
		return b.createDraftOneTimeReminder(ctx, chatID, draft, remindAt)
	case telegramui.RepeatWeekdays:
		return b.createDraftWeekdaysReminder(ctx, chatID, draft, remindAt, telegrammodel.WeekdaysMaskWorkdays)
	case telegramui.RepeatCustomWeekdays:
		return b.createDraftWeekdaysReminder(ctx, chatID, draft, remindAt, draft.weekdaysMask)
	case telegramui.RepeatCustomDates:
		return b.createDraftCustomDatesReminder(ctx, chatID, draft)
	case telegramui.RepeatCustomInterval:
		return b.createDraftIntervalReminder(ctx, chatID, draft, remindAt, draft.intervalDays)
	default:
		intervalDays, ok := intervalDaysForRepeat(draft.repeat)
		if !ok {
			return telegrammodel.ReminderView{}, fmt.Errorf("%w: %s", ErrUnsupportedReminderRepeat, draft.repeat)
		}

		return b.createDraftIntervalReminder(ctx, chatID, draft, remindAt, intervalDays)
	}
}

func (b *ReminderBot) createDraftOneTimeReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
	remindAt time.Time,
) (telegrammodel.ReminderView, error) {
	reminder, err := b.service.CreateReminder(ctx, chatID, draft.title, draft.description, remindAt)
	if err != nil {
		return telegrammodel.ReminderView{}, fmt.Errorf("create one-time reminder: %w", err)
	}

	return reminder, nil
}

func (b *ReminderBot) createDraftWeekdaysReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
	remindAt time.Time,
	weekdaysMask int,
) (telegrammodel.ReminderView, error) {
	reminder, err := b.service.CreateWeekdaysReminder(
		ctx,
		chatID,
		draft.title,
		draft.description,
		remindAt,
		weekdaysMask,
	)
	if err != nil {
		return telegrammodel.ReminderView{}, fmt.Errorf("create weekdays reminder: %w", err)
	}

	return reminder, nil
}

func (b *ReminderBot) createDraftCustomDatesReminder(
	ctx context.Context,
	chatID int64,
	draft reminderDraft,
) (telegrammodel.ReminderView, error) {
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
		return telegrammodel.ReminderView{}, fmt.Errorf("create custom dates reminder: %w", err)
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
) (telegrammodel.ReminderView, error) {
	reminder, err := b.service.CreateIntervalReminder(
		ctx,
		chatID,
		draft.title,
		draft.description,
		remindAt,
		intervalDays,
	)
	if err != nil {
		return telegrammodel.ReminderView{}, fmt.Errorf("create interval reminder: %w", err)
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
	case telegramui.RepeatNone:
		draft.customDates = nil
		draft.weekdaysMask = 0
		draft.intervalDays = 0
		draft.waitingFor = ""
		draft.screen = telegramui.ScreenMain
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

	if field == telegramui.FieldIntrusiveInterval {
		draft := b.draftByChat[chatID]
		draft.waitingFor = field
		draft.notice = ""
		b.draftByChat[chatID] = draft

		return
	}

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

	draft, exists := b.draftByChat[chatID]
	if !exists {
		return false
	}

	if draft.waitingFor == telegramui.FieldIntrusiveInterval &&
		draft.title == "" &&
		draft.description == "" &&
		draft.date.IsZero() {
		return false
	}

	return true
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

func (b *ReminderBot) setReminderListState(chatID int64, list reminderListState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.listByChat == nil {
		b.listByChat = make(map[int64]reminderListState)
	}

	b.listByChat[chatID] = list
}

func (b *ReminderBot) reminderListState(chatID int64) reminderListState {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.listByChat[chatID]
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
	settings, err := b.userSettings(ctx, chatID)
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
		Kind:          draft.kind,
		Title:         draft.title,
		Description:   draft.description,
		Notice:        draft.notice,
		ShoppingItems: draft.shoppingItems,
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
