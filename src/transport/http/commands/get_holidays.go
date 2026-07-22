//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetHolidaysCommand struct {
	req     PaginationRequest
	service *service.ReminderService
}

func NewGetHolidaysCommand(svc *service.ReminderService) *GetHolidaysCommand {
	return &GetHolidaysCommand{
		service: svc,
	}
}

func (c *GetHolidaysCommand) Method() string {
	return http.MethodGet
}

func (c *GetHolidaysCommand) Path() string {
	return "/holidays"
}

func (c *GetHolidaysCommand) Clone() apphttp.Command {
	return &GetHolidaysCommand{
		service: c.service,
	}
}

func (c *GetHolidaysCommand) Parse(ctx *gin.Context) error {
	req, err := parsePaginationRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *GetHolidaysCommand) Apply(ctx *gin.Context) (any, error) {
	holidays, err := c.service.FindHolidaysByChat(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.Limit,
		c.req.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("find holidays by chat: %w", err)
	}

	return HolidayListResponse{
		Holidays: mapHolidaysViewToResponse(holidays),
	}, nil
}
