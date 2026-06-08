package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/config"
	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/logger"
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

	return &Sender{bot: bot, cfg: cfg}, nil
}

type Sender struct {
	bot *telego.Bot
	cfg config.Telegram
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

	params := tu.Message(tu.ID(reminder.TelegramChatID), text)
	if reminder.IntrusiveEnabled {
		params.WithReplyMarkup(tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Готово").
					WithCallbackData("reminder:done:"+strconv.FormatInt(reminder.ID, 10)),
				tu.InlineKeyboardButton("Отменить").
					WithCallbackData("reminder:cancel_delivered:"+strconv.FormatInt(reminder.ID, 10)),
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

	message, err := s.bot.SendMessage(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("send telegram reminder %d: %w", reminder.ID, err)
	}

	if message == nil {
		return 0, nil
	}

	return message.MessageID, nil
}
