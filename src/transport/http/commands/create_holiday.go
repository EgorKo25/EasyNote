package commands

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type CreateHolidayCommand struct {
	req     CreateHolidayRequest
	service *service.ReminderService
}

func NewCreateHolidayCommand(svc *service.ReminderService) *CreateHolidayCommand {
	return &CreateHolidayCommand{
		service: svc,
	}
}

func (c *CreateHolidayCommand) Method() string {
	return http.MethodPost
}

func (c *CreateHolidayCommand) Path() string {
	return "/holidays"
}

func (c *CreateHolidayCommand) Clone() apphttp.Command {
	return &CreateHolidayCommand{
		service: c.service,
	}
}

func (c *CreateHolidayCommand) Parse(ctx *gin.Context) error {
	var req CreateHolidayRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
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
	req.HolidayDateValue = holidayDate
	c.req = req

	return nil
}

func (c *CreateHolidayCommand) Apply(ctx *gin.Context) (any, error) {
	holiday, err := c.service.CreateHoliday(
		ctx.Request.Context(),
		mapCreateHolidayRequestToDatabaseRequest(c.req),
	)
	if err != nil {
		return nil, err
	}

	return mapHolidayViewToResponse(holiday), nil
}
