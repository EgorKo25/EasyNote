package commands

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type UpdateHolidayCommand struct {
	req     UpdateHolidayRequest
	service *service.ReminderService
}

func NewUpdateHolidayCommand(svc *service.ReminderService) *UpdateHolidayCommand {
	return &UpdateHolidayCommand{
		service: svc,
	}
}

func (c *UpdateHolidayCommand) Method() string {
	return http.MethodPatch
}

func (c *UpdateHolidayCommand) Path() string {
	return "/holidays/:id"
}

func (c *UpdateHolidayCommand) Clone() apphttp.Command {
	return &UpdateHolidayCommand{
		service: c.service,
	}
}

func (c *UpdateHolidayCommand) Parse(ctx *gin.Context) error {
	var req UpdateHolidayRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	holidayID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || holidayID <= 0 {
		return apphttp.ErrInvalidID
	}

	if strings.TrimSpace(req.HolidayName) == "" {
		return apphttp.ErrHolidayNameEmpty
	}

	if strings.TrimSpace(req.RecipientName) == "" {
		return apphttp.ErrHolidayRecipientNameEmpty
	}

	holidayDate, err := time.Parse(time.DateOnly, req.HolidayDate)
	if err != nil {
		return apphttp.ErrHolidayDateInvalid
	}

	if req.RemindAt.IsZero() {
		return apphttp.ErrHolidayRemindAtEmpty
	}

	req.TelegramChatID = telegramChatID
	req.HolidayID = holidayID
	req.HolidayDateValue = holidayDate
	c.req = req

	return nil
}

func (c *UpdateHolidayCommand) Apply(ctx *gin.Context) (any, error) {
	holiday, err := c.service.UpdateHoliday(
		ctx.Request.Context(),
		mapUpdateHolidayRequestToDatabaseRequest(c.req),
	)
	if err != nil {
		return nil, err
	}

	return mapHolidayViewToResponse(holiday), nil
}
