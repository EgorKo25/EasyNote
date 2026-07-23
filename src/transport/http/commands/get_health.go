package commands

import (
	"net/http"

	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetHealthCommand struct{}

const getHealthPath = "/health"

func NewGetHealthCommand() *GetHealthCommand {
	return &GetHealthCommand{}
}

func (c *GetHealthCommand) Method() string {
	return http.MethodGet
}

func (c *GetHealthCommand) Path() string {
	return getHealthPath
}

func (c *GetHealthCommand) Clone() apphttp.Command {
	return &GetHealthCommand{}
}

func (c *GetHealthCommand) Parse(ctx *gin.Context) error {
	return nil
}

func (c *GetHealthCommand) Apply(ctx *gin.Context) (any, error) {
	return HealthResponse{Status: "ok"}, nil
}
