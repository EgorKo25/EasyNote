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

type CompleteReminderCommand struct {
	req     AcknowledgeReminderRequest
	service *service.ReminderService
}

const completeReminderPath = "/reminders/:id/complete"

func NewCompleteReminderCommand(svc *service.ReminderService) *CompleteReminderCommand {
	return &CompleteReminderCommand{
		service: svc,
	}
}

func (c *CompleteReminderCommand) Method() string {
	return http.MethodPost
}

func (c *CompleteReminderCommand) Path() string {
	return completeReminderPath
}

func (c *CompleteReminderCommand) Clone() apphttp.Command {
	return &CompleteReminderCommand{
		service: c.service,
	}
}

func (c *CompleteReminderCommand) Parse(ctx *gin.Context) error {
	req, err := parseAcknowledgeReminderRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *CompleteReminderCommand) Apply(ctx *gin.Context) (any, error) {
	if err := c.service.AcknowledgeReminder(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.ReminderID,
		domain.ReminderStatusCompleted,
	); err != nil {
		return nil, fmt.Errorf("complete reminder: %w", err)
	}

	return EmptyResponse{}, nil
}
