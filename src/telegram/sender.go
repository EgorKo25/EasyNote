package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
	telegramui "github.com/EgorKo25/EasyNote/src/telegram/ui"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

var errEmptyToken = errors.New("telegram token is empty")

func NewSender(ctx context.Context, cfg config.Telegram) (*Sender, error) {
	token := os.Getenv(cfg.TokenEnv)
	if token == "" {
		return nil, fmt.Errorf("%w: env %s", errEmptyToken, cfg.TokenEnv)
	}

	options := []telego.BotOption{
		telego.WithAPIServer(cfg.APIBaseURL),
		telego.WithHTTPClient(&http.Client{
			Timeout: cfg.RequestTimeout,
		}),
		telego.WithLogger(logger.Get("telego")),
	}

	if cfg.Warnings {
		options = append(options, telego.WithWarnings())
	}

	if cfg.HealthCheck {
		options = append(options, telego.WithHealthCheck(ctx))
	}

	bot, err := telego.NewBot(token, options...)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &Sender{bot: bot, cfg: cfg, mainMenuByChat: make(map[int64]int)}, nil
}

type Sender struct {
	bot            *telego.Bot
	cfg            config.Telegram
	mainMenuMu     sync.Mutex
	mainMenuByChat map[int64]int
}

func (s *Sender) SendReminder(ctx context.Context, reminder database.ScheduledReminder) (int, error) {
	text := reminder.Title
	if reminder.Description != "" {
		text = fmt.Sprintf("%s\n\n%s", reminder.Title, reminder.Description)
	}

	if reminder.TelegramMessageID != nil {
		err := s.bot.DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    tu.ID(reminder.TelegramChatID),
			MessageID: *reminder.TelegramMessageID,
		})
		if err != nil {
			logger.Get("telegram/sender").Warn("cannot delete previous reminder message", logger.Any("error", err))
		}
	}

	if err := s.refreshMainKeyboard(ctx, reminder); err != nil {
		return 0, fmt.Errorf("send telegram main menu keyboard before reminder %d: %w", reminder.ID, err)
	}

	params := s.reminderMessageParams(reminder, text)

	message, err := s.bot.SendMessage(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("send telegram reminder %d: %w", reminder.ID, err)
	}

	if message == nil {
		return 0, nil
	}

	return message.MessageID, nil
}

func (s *Sender) refreshMainKeyboard(ctx context.Context, reminder database.ScheduledReminder) error {
	if previousMenuID := s.mainMenuMessageID(reminder.TelegramChatID); previousMenuID != 0 {
		err := s.bot.DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    tu.ID(reminder.TelegramChatID),
			MessageID: previousMenuID,
		})
		if err != nil {
			logger.Get("telegram/sender").Warn("cannot delete previous main menu message", logger.Any("error", err))
		}
	}

	menuMessage, err := s.bot.SendMessage(ctx,
		tu.Message(tu.ID(reminder.TelegramChatID), "Главное меню").WithReplyMarkup(telegramui.MainKeyboard()),
	)
	if err != nil {
		return fmt.Errorf("send telegram main menu keyboard message: %w", err)
	}

	if menuMessage != nil {
		s.setMainMenuMessageID(reminder.TelegramChatID, menuMessage.MessageID)
	}

	return nil
}

func (s *Sender) mainMenuMessageID(chatID int64) int {
	s.mainMenuMu.Lock()
	defer s.mainMenuMu.Unlock()

	return s.mainMenuByChat[chatID]
}

func (s *Sender) setMainMenuMessageID(chatID int64, messageID int) {
	s.mainMenuMu.Lock()
	defer s.mainMenuMu.Unlock()

	if s.mainMenuByChat == nil {
		s.mainMenuByChat = make(map[int64]int)
	}

	s.mainMenuByChat[chatID] = messageID
}

func (s *Sender) reminderMessageParams(reminder database.ScheduledReminder, text string) *telego.SendMessageParams {
	params := tu.Message(tu.ID(reminder.TelegramChatID), text)
	if reminder.IntrusiveEnabled {
		params.WithReplyMarkup(tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Готово").
					WithCallbackData(telegramui.CallbackReminderDone+strconv.FormatInt(reminder.ID, 10)),
				tu.InlineKeyboardButton("Отменить").
					WithCallbackData(telegramui.CallbackReminderCancelDelivered+strconv.FormatInt(reminder.ID, 10)),
			),
		))
	}

	if s.cfg.ParseMode != "" {
		params.WithParseMode(s.cfg.ParseMode)
	}

	if s.cfg.DisableNotification {
		params.WithDisableNotification()
	}

	if s.cfg.ProtectContent {
		params.WithProtectContent()
	}

	return params
}
