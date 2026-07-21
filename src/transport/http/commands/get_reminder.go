package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type GetReminderCommand struct {
	req     ReminderRequest
	service *service.ReminderService
}

func NewGetReminderCommand(svc *service.ReminderService) *GetReminderCommand {
	return &GetReminderCommand{
		service: svc,
	}
}

func (c *GetReminderCommand) Method() string {
	return http.MethodGet
}

func (c *GetReminderCommand) Path() string {
	return "/reminders/:id"
}

func (c *GetReminderCommand) Clone() apphttp.Command {
	return &GetReminderCommand{
		service: c.service,
	}
}

func (c *GetReminderCommand) Parse(ctx *gin.Context) error {
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

func (c *GetReminderCommand) Apply(ctx *gin.Context) (any, error) {
	reminder, err := c.service.ReminderByID(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.ReminderID,
	)
	if err != nil {
		return nil, err
	}

	return mapReminderViewToResponse(reminder), nil
}
