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

		attrs := requestLogAttrs(ctx, start)
		requestErr := lastRequestError(ctx)
		logRequest(log, ctx.Writer.Status(), requestErr, attrs)
	}
}

func requestLogAttrs(ctx *gin.Context, start time.Time) []logger.Attr {
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

	return appendTelegramChatID(attrs, ctx)
}

func appendTelegramChatID(attrs []logger.Attr, ctx *gin.Context) []logger.Attr {
	value, exists := ctx.Get(apphttp.TelegramChatIDContextKey)
	if !exists {
		return attrs
	}

	telegramChatID, ok := value.(int64)
	if !ok {
		return attrs
	}

	return append(attrs, logger.Int64("telegram_chat_id", telegramChatID))
}

func lastRequestError(ctx *gin.Context) error {
	if len(ctx.Errors) == 0 {
		return nil
	}

	return ctx.Errors.Last().Err
}

func logRequest(log logger.Logger, status int, requestErr error, attrs []logger.Attr) {
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
