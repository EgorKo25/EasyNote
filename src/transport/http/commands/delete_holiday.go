//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type DeleteHolidayCommand struct {
	req     HolidayRequest
	service *service.ReminderService
}

const deleteHolidayPath = "/holidays/:id"

func NewDeleteHolidayCommand(svc *service.ReminderService) *DeleteHolidayCommand {
	return &DeleteHolidayCommand{
		service: svc,
	}
}

func (c *DeleteHolidayCommand) Method() string {
	return http.MethodDelete
}

func (c *DeleteHolidayCommand) Path() string {
	return deleteHolidayPath
}

func (c *DeleteHolidayCommand) Clone() apphttp.Command {
	return &DeleteHolidayCommand{
		service: c.service,
	}
}

func (c *DeleteHolidayCommand) Parse(ctx *gin.Context) error {
	req, err := parseHolidayRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *DeleteHolidayCommand) Apply(ctx *gin.Context) (any, error) {
	if err := c.service.DeleteHoliday(ctx.Request.Context(), c.req.TelegramChatID, c.req.HolidayID); err != nil {
		return nil, fmt.Errorf("delete holiday: %w", err)
	}

	return EmptyResponse{}, nil
}
