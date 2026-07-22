package commands

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type UpdateHolidayCommand struct {
	req     UpdateHolidayRequest
	service *service.ReminderService
}

const updateHolidayPath = "/holidays/:id"

func NewUpdateHolidayCommand(svc *service.ReminderService) *UpdateHolidayCommand {
	return &UpdateHolidayCommand{
		service: svc,
	}
}

func (c *UpdateHolidayCommand) Method() string {
	return http.MethodPatch
}

func (c *UpdateHolidayCommand) Path() string {
	return updateHolidayPath
}

func (c *UpdateHolidayCommand) Clone() apphttp.Command {
	return &UpdateHolidayCommand{
		service: c.service,
	}
}

func (c *UpdateHolidayCommand) Parse(ctx *gin.Context) error {
	var req UpdateHolidayRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %w", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return err
	}

	holidayID, err := parsePositiveInt64Param(ctx, "id", apphttp.ErrInvalidID)
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
	req.HolidayID = holidayID
	req.HolidayDateValue = holidayDate
	c.req = req

	return nil
}

func (c *UpdateHolidayCommand) Apply(ctx *gin.Context) (any, error) {
	holiday, err := c.service.UpdateHoliday(
		ctx.Request.Context(),
		mapUpdateHolidayRequestToDomainRequest(c.req),
	)
	if err != nil {
		return nil, fmt.Errorf("update holiday: %w", err)
	}

	return mapHolidayViewToResponse(holiday), nil
}
