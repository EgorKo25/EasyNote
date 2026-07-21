package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/EgorKo25/EasyNote/src/logger"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

var errInternalServerErrorStatus = errors.New("http response status is internal server error")

func RequestLoggerMiddleware() gin.HandlerFunc {
	log := logger.Get("transport/http")

	return func(ctx *gin.Context) {
		start := time.Now()

		ctx.Next()

		path := ctx.FullPath()
		if path == "" {
			path = ctx.Request.URL.Path
		}

		status := ctx.Writer.Status()
		attrs := []logger.Attr{
			logger.String("method", ctx.Request.Method),
			logger.String("path", path),
			logger.Int("status", status),
			logger.String("duration", time.Since(start).String()),
			logger.String("client_ip", ctx.ClientIP()),
		}

		value, exists := ctx.Get(apphttp.TelegramChatIDContextKey)
		if exists {
			telegramChatID, ok := value.(int64)
			if ok {
				attrs = append(attrs, logger.Int64("telegram_chat_id", telegramChatID))
			}
		}

		var requestErr error
		if len(ctx.Errors) > 0 {
			requestErr = ctx.Errors.Last().Err
		}

		switch {
		case requestErr != nil && status >= http.StatusInternalServerError:
			log.Error("http request failed", requestErr, attrs...)
		case requestErr != nil:
			attrs = append(attrs, logger.String("error", requestErr.Error()))
			log.Warn("http request failed", attrs...)
		case status >= http.StatusInternalServerError:
			log.Error("http request failed", errInternalServerErrorStatus, attrs...)
		case status >= http.StatusBadRequest:
			log.Warn("http request failed", attrs...)
		default:
			log.Info("http request handled", attrs...)
		}
	}
}
