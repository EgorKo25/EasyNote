package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		headers := ctx.Writer.Header()
		headers.Set("Access-Control-Allow-Origin", "*")
		headers.Set("Access-Control-Allow-Headers", "Content-Type, Accept, X-Telegram-Chat-ID")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)

			return
		}

		ctx.Next()
	}
}
