package middleware

import (
	"strconv"

	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/apperrors"
	"github.com/gin-gonic/gin"
)

func TelegramChatID(ctx *gin.Context) (int64, error) {
	value, exists := ctx.Get(apphttp.TelegramChatIDContextKey)
	if !exists {
		return 0, apphttp.ErrTelegramChatIDMissing
	}

	telegramChatID, ok := value.(int64)
	if !ok {
		return 0, apphttp.ErrTelegramChatIDInvalid
	}

	return telegramChatID, nil
}

func TelegramChatIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		rawTelegramChatID := ctx.GetHeader(apphttp.TelegramChatIDHeader)
		if rawTelegramChatID == "" {
			requestErr := apphttp.ErrTelegramChatIDMissing
			_ = ctx.Error(requestErr)
			apperrors.WriteError(ctx, requestErr)
			ctx.Abort()

			return
		}

		telegramChatID, err := strconv.ParseInt(rawTelegramChatID, 10, 64)
		if err != nil {
			requestErr := apphttp.ErrTelegramChatIDInvalid
			_ = ctx.Error(requestErr)
			apperrors.WriteError(ctx, requestErr)
			ctx.Abort()

			return
		}

		ctx.Set(apphttp.TelegramChatIDContextKey, telegramChatID)
		ctx.Next()
	}
}
