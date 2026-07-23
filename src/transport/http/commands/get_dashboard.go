//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetDashboardCommand struct {
	req     TelegramChatRequest
	service *service.ReminderService
}

const getDashboardPath = "/dashboard"

func NewGetDashboardCommand(svc *service.ReminderService) *GetDashboardCommand {
	return &GetDashboardCommand{
		service: svc,
	}
}

func (c *GetDashboardCommand) Method() string {
	return http.MethodGet
}

func (c *GetDashboardCommand) Path() string {
	return getDashboardPath
}

func (c *GetDashboardCommand) Clone() apphttp.Command {
	return &GetDashboardCommand{
		service: c.service,
	}
}

func (c *GetDashboardCommand) Parse(ctx *gin.Context) error {
	req, err := parseTelegramChatRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *GetDashboardCommand) Apply(ctx *gin.Context) (any, error) {
	stats, err := c.service.ReminderStatsByChat(ctx.Request.Context(), c.req.TelegramChatID)
	if err != nil {
		return nil, fmt.Errorf("get dashboard: %w", err)
	}

	return DashboardResponse{
		Stats: mapReminderStatsToDTO(stats),
	}, nil
}
