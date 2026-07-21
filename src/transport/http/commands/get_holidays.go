package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
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
	var req PaginationRequest

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	req.TelegramChatID = telegramChatID
	req.Limit = 10
	req.Offset = 0

	if value := ctx.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return apphttp.ErrInvalidOffset
		}
		req.Offset = offset
	}

	if value := ctx.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			return apphttp.ErrInvalidLimit
		}
		req.Limit = limit
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
		return nil, err
	}

	return HolidayListResponse{
		Holidays: mapHolidaysViewToResponse(holidays),
	}, nil
}
