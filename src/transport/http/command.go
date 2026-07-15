package apphttp

import "github.com/gin-gonic/gin"

type Command interface {
	Method() string
	Path() string

	Clone() Command

	Parse(ctx *gin.Context) error
	Apply(ctx *gin.Context) (any, error)
}
