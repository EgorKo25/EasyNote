//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type DeleteReminderCommand struct {
	req     ReminderRequest
	service *service.ReminderService
}

const deleteReminderPath = "/reminders/:id"

func NewDeleteReminderCommand(svc *service.ReminderService) *DeleteReminderCommand {
	return &DeleteReminderCommand{
		service: svc,
	}
}

func (c *DeleteReminderCommand) Method() string {
	return http.MethodDelete
}

func (c *DeleteReminderCommand) Path() string {
	return deleteReminderPath
}

func (c *DeleteReminderCommand) Clone() apphttp.Command {
	return &DeleteReminderCommand{
		service: c.service,
	}
}

func (c *DeleteReminderCommand) Parse(ctx *gin.Context) error {
	req, err := parseReminderRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *DeleteReminderCommand) Apply(ctx *gin.Context) (any, error) {
	if err := c.service.DeleteReminder(ctx.Request.Context(), c.req.TelegramChatID, c.req.ReminderID); err != nil {
		return nil, fmt.Errorf("delete reminder: %w", err)
	}

	return EmptyResponse{}, nil
}
