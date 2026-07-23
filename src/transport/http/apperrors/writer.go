package apperrors

import "github.com/gin-gonic/gin"

func WriteError(ctx *gin.Context, err error) {
	response := mapErrorResponse(err)

	ctx.JSON(response.Error.Code, response)
}
