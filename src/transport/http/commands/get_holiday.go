package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type GetHolidayCommand struct {
	req     HolidayRequest
	service *service.ReminderService
}

func NewGetHolidayCommand(svc *service.ReminderService) *GetHolidayCommand {
	return &GetHolidayCommand{
		service: svc,
	}
}

func (c *GetHolidayCommand) Method() string {
	return http.MethodGet
}

func (c *GetHolidayCommand) Path() string {
	return "/holidays/:id"
}

func (c *GetHolidayCommand) Clone() apphttp.Command {
	return &GetHolidayCommand{
		service: c.service,
	}
}

func (c *GetHolidayCommand) Parse(ctx *gin.Context) error {
	var req HolidayRequest

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	holidayID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || holidayID <= 0 {
		return apphttp.ErrInvalidID
	}

	req.TelegramChatID = telegramChatID
	req.HolidayID = holidayID
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
		return nil, err
	}

	return mapHolidayViewToResponse(holiday), nil
}
