package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type CancelReminderCommand struct {
	req     AcknowledgeReminderRequest
	service *service.ReminderService
}

func NewCancelReminderCommand(svc *service.ReminderService) *CancelReminderCommand {
	return &CancelReminderCommand{
		service: svc,
	}
}

func (c *CancelReminderCommand) Method() string {
	return http.MethodPost
}

func (c *CancelReminderCommand) Path() string {
	return "/reminders/:id/cancel"
}

func (c *CancelReminderCommand) Clone() apphttp.Command {
	return &CancelReminderCommand{
		service: c.service,
	}
}

func (c *CancelReminderCommand) Parse(ctx *gin.Context) error {
	var req AcknowledgeReminderRequest

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

func (c *CancelReminderCommand) Apply(ctx *gin.Context) (any, error) {
	if err := c.service.AcknowledgeReminder(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.ReminderID,
		database.ReminderStatusCanceled,
	); err != nil {
		return nil, err
	}

	return EmptyResponse{}, nil
}
