package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type UpdateReminderCommand struct {
	req     UpdateReminderRequest
	service *service.ReminderService
}

const updateReminderPath = "/reminders/:id"

func NewUpdateReminderCommand(svc *service.ReminderService) *UpdateReminderCommand {
	return &UpdateReminderCommand{
		service: svc,
	}
}

func (c *UpdateReminderCommand) Method() string {
	return http.MethodPatch
}

func (c *UpdateReminderCommand) Path() string {
	return updateReminderPath
}

func (c *UpdateReminderCommand) Clone() apphttp.Command {
	return &UpdateReminderCommand{
		service: c.service,
	}
}

func (c *UpdateReminderCommand) Parse(ctx *gin.Context) error {
	var req UpdateReminderRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %w", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return err
	}

	reminderID, err := parsePositiveInt64Param(ctx, "id", apphttp.ErrInvalidID)
	if err != nil {
		return err
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
		return nil, fmt.Errorf("update reminder: %w", err)
	}

	return mapReminderViewToResponse(reminder), nil
}
