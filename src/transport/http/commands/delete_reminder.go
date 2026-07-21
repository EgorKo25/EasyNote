package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type DeleteReminderCommand struct {
	req     ReminderRequest
	service *service.ReminderService
}

func NewDeleteReminderCommand(svc *service.ReminderService) *DeleteReminderCommand {
	return &DeleteReminderCommand{
		service: svc,
	}
}

func (c *DeleteReminderCommand) Method() string {
	return http.MethodDelete
}

func (c *DeleteReminderCommand) Path() string {
	return "/reminders/:id"
}

func (c *DeleteReminderCommand) Clone() apphttp.Command {
	return &DeleteReminderCommand{
		service: c.service,
	}
}

func (c *DeleteReminderCommand) Parse(ctx *gin.Context) error {
	var req ReminderRequest

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

func (c *DeleteReminderCommand) Apply(ctx *gin.Context) (any, error) {
	if err := c.service.DeleteReminder(ctx.Request.Context(), c.req.TelegramChatID, c.req.ReminderID); err != nil {
		return nil, err
	}

	return EmptyResponse{}, nil
}
