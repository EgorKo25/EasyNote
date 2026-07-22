package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetReminderCommand struct {
	req     ReminderRequest
	service *service.ReminderService
}

const getReminderPath = "/reminders/:id"

func NewGetReminderCommand(svc *service.ReminderService) *GetReminderCommand {
	return &GetReminderCommand{
		service: svc,
	}
}

func (c *GetReminderCommand) Method() string {
	return http.MethodGet
}

func (c *GetReminderCommand) Path() string {
	return getReminderPath
}

func (c *GetReminderCommand) Clone() apphttp.Command {
	return &GetReminderCommand{
		service: c.service,
	}
}

func (c *GetReminderCommand) Parse(ctx *gin.Context) error {
	req, err := parseReminderRequest(ctx)
	if err != nil {
		return err
	}

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
		return nil, fmt.Errorf("get reminder by id: %w", err)
	}

	return mapReminderViewToResponse(reminder), nil
}
