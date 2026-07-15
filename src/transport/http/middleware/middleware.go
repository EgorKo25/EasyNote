package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const TelegramChatIDHeader = "X-Telegram-Chat-ID"
const TelegramChatIDContextKey = "telegram_chat_id"

func TelegramChatID(ctx *gin.Context) (int64, error) {
	value, exists := ctx.Get(TelegramChatIDContextKey)
	if !exists {
		return 0, ErrTelegramChatIDMissing
	}

	telegramChatID, ok := value.(int64)
	if !ok {
		return 0, ErrTelegramChatIDInvalid
	}

	return telegramChatID, nil
}

func TelegramChatIDMiddleware(writeError func(ctx *gin.Context, err error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		rawTelegramChatID := ctx.GetHeader(TelegramChatIDHeader)
		if rawTelegramChatID == "" {
			writeError(ctx, ErrTelegramChatIDMissing)
			ctx.Abort()

			return
		}

		telegramChatID, err := strconv.ParseInt(rawTelegramChatID, 10, 64)
		if err != nil {
			writeError(ctx, ErrTelegramChatIDInvalid)
			ctx.Abort()

			return
		}

		ctx.Set(TelegramChatIDContextKey, telegramChatID)
		ctx.Next()
	}
}
