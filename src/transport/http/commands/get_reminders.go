//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetRemindersCommand struct {
	req     PaginationRequest
	service *service.ReminderService
}

const getRemindersPath = "/reminders"

func NewGetRemindersCommand(svc *service.ReminderService) *GetRemindersCommand {
	return &GetRemindersCommand{
		service: svc,
	}
}

func (c *GetRemindersCommand) Method() string {
	return http.MethodGet
}

func (c *GetRemindersCommand) Path() string {
	return getRemindersPath
}

func (c *GetRemindersCommand) Clone() apphttp.Command {
	return &GetRemindersCommand{
		service: c.service,
	}
}

func (c *GetRemindersCommand) Parse(ctx *gin.Context) error {
	req, err := parsePaginationRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *GetRemindersCommand) Apply(ctx *gin.Context) (any, error) {
	reminders, err := c.service.FindRemindersByChat(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.Limit,
		c.req.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("find reminders by chat: %w", err)
	}

	return ReminderListResponse{
		Reminders: mapRemindersViewToResponse(reminders),
	}, nil
}
