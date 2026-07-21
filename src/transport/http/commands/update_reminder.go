package commands

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type UpdateReminderCommand struct {
	req     UpdateReminderRequest
	service *service.ReminderService
}

func NewUpdateReminderCommand(svc *service.ReminderService) *UpdateReminderCommand {
	return &UpdateReminderCommand{
		service: svc,
	}
}

func (c *UpdateReminderCommand) Method() string {
	return http.MethodPatch
}

func (c *UpdateReminderCommand) Path() string {
	return "/reminders/:id"
}

func (c *UpdateReminderCommand) Clone() apphttp.Command {
	return &UpdateReminderCommand{
		service: c.service,
	}
}

func (c *UpdateReminderCommand) Parse(ctx *gin.Context) error {
	var req UpdateReminderRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	reminderID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || reminderID <= 0 {
		return apphttp.ErrInvalidID
	}

	req.TelegramChatID = telegramChatID
	req.ReminderID = reminderID
	c.req = req

	return nil
}

func (c *UpdateReminderCommand) Apply(ctx *gin.Context) (any, error) {
	reminder, err := c.service.UpdateReminder(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.ReminderID,
		c.req.Title,
		c.req.Description,
		c.req.RemindAt,
	)
	if err != nil {
		return nil, err
	}

	return mapReminderViewToResponse(reminder), nil
}
