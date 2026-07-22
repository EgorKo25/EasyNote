//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/domain"
	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
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
	req, err := parseAcknowledgeReminderRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *CancelReminderCommand) Apply(ctx *gin.Context) (any, error) {
	if err := c.service.AcknowledgeReminder(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.ReminderID,
		domain.ReminderStatusCanceled,
	); err != nil {
		return nil, fmt.Errorf("cancel reminder: %w", err)
	}

	return EmptyResponse{}, nil
}
