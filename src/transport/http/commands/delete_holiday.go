package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type DeleteHolidayCommand struct {
	req     HolidayRequest
	service *service.ReminderService
}

func NewDeleteHolidayCommand(svc *service.ReminderService) *DeleteHolidayCommand {
	return &DeleteHolidayCommand{
		service: svc,
	}
}

func (c *DeleteHolidayCommand) Method() string {
	return http.MethodDelete
}

func (c *DeleteHolidayCommand) Path() string {
	return "/holidays/:id"
}

func (c *DeleteHolidayCommand) Clone() apphttp.Command {
	return &DeleteHolidayCommand{
		service: c.service,
	}
}

func (c *DeleteHolidayCommand) Parse(ctx *gin.Context) error {
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

func (c *DeleteHolidayCommand) Apply(ctx *gin.Context) (any, error) {
	if err := c.service.DeleteHoliday(ctx.Request.Context(), c.req.TelegramChatID, c.req.HolidayID); err != nil {
		return nil, err
	}

	return EmptyResponse{}, nil
}
