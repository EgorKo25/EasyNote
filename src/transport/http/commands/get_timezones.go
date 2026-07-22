package commands

import (
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetTimezonesCommand struct{}

const getTimezonesPath = "/timezones"

func NewGetTimezonesCommand() *GetTimezonesCommand {
	return &GetTimezonesCommand{}
}

func (c *GetTimezonesCommand) Method() string {
	return http.MethodGet
}

func (c *GetTimezonesCommand) Path() string {
	return getTimezonesPath
}

func (c *GetTimezonesCommand) Clone() apphttp.Command {
	return &GetTimezonesCommand{}
}

func (c *GetTimezonesCommand) Parse(ctx *gin.Context) error {
	return nil
}

func (c *GetTimezonesCommand) Apply(ctx *gin.Context) (any, error) {
	return TimezoneListResponse{
		Timezones: mapTimezoneOptionsToResponse(service.TimezoneOptions()),
	}, nil
}
