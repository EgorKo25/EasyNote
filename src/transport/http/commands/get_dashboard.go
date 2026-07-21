package commands

import (
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type GetDashboardCommand struct {
	req     TelegramChatRequest
	service *service.ReminderService
}

func NewGetDashboardCommand(svc *service.ReminderService) *GetDashboardCommand {
	return &GetDashboardCommand{
		service: svc,
	}
}

func (c *GetDashboardCommand) Method() string {
	return http.MethodGet
}

func (c *GetDashboardCommand) Path() string {
	return "/dashboard"
}

func (c *GetDashboardCommand) Clone() apphttp.Command {
	return &GetDashboardCommand{
		service: c.service,
	}
}

func (c *GetDashboardCommand) Parse(ctx *gin.Context) error {
	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	c.req = TelegramChatRequest{TelegramChatID: telegramChatID}

	return nil
}

func (c *GetDashboardCommand) Apply(ctx *gin.Context) (any, error) {
	stats, err := c.service.ReminderStatsByChat(ctx.Request.Context(), c.req.TelegramChatID)
	if err != nil {
		return nil, err
	}

	return DashboardResponse{
		Stats: mapReminderStatsToDTO(stats),
	}, nil
}
