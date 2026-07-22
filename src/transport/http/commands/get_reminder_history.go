//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetReminderHistoryCommand struct {
	req     PaginationRequest
	service *service.ReminderService
}

func NewGetReminderHistoryCommand(svc *service.ReminderService) *GetReminderHistoryCommand {
	return &GetReminderHistoryCommand{
		service: svc,
	}
}

func (c *GetReminderHistoryCommand) Method() string {
	return http.MethodGet
}

func (c *GetReminderHistoryCommand) Path() string {
	return "/reminders/history"
}

func (c *GetReminderHistoryCommand) Clone() apphttp.Command {
	return &GetReminderHistoryCommand{
		service: c.service,
	}
}

func (c *GetReminderHistoryCommand) Parse(ctx *gin.Context) error {
	req, err := parsePaginationRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *GetReminderHistoryCommand) Apply(ctx *gin.Context) (any, error) {
	reminders, err := c.service.FindReminderHistoryByChat(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.Limit,
		c.req.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("find reminder history by chat: %w", err)
	}

	return ReminderListResponse{
		Reminders: mapRemindersViewToResponse(reminders),
	}, nil
}
