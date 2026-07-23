//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetHolidayCommand struct {
	req     HolidayRequest
	service *service.ReminderService
}

const getHolidayPath = "/holidays/:id"

func NewGetHolidayCommand(svc *service.ReminderService) *GetHolidayCommand {
	return &GetHolidayCommand{
		service: svc,
	}
}

func (c *GetHolidayCommand) Method() string {
	return http.MethodGet
}

func (c *GetHolidayCommand) Path() string {
	return getHolidayPath
}

func (c *GetHolidayCommand) Clone() apphttp.Command {
	return &GetHolidayCommand{
		service: c.service,
	}
}

func (c *GetHolidayCommand) Parse(ctx *gin.Context) error {
	req, err := parseHolidayRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *GetHolidayCommand) Apply(ctx *gin.Context) (any, error) {
	holiday, err := c.service.HolidayByID(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.HolidayID,
	)
	if err != nil {
		return nil, fmt.Errorf("get holiday by id: %w", err)
	}

	return mapHolidayViewToResponse(holiday), nil
}
