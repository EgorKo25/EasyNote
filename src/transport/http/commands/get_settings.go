package commands

import (
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type GetSettingsCommand struct {
	req     TelegramChatRequest
	service *service.ReminderService
}

func NewGetSettingsCommand(svc *service.ReminderService) *GetSettingsCommand {
	return &GetSettingsCommand{
		service: svc,
	}
}

func (c *GetSettingsCommand) Method() string {
	return http.MethodGet
}

func (c *GetSettingsCommand) Path() string {
	return "/settings"
}

func (c *GetSettingsCommand) Clone() apphttp.Command {
	return &GetSettingsCommand{
		service: c.service,
	}
}

func (c *GetSettingsCommand) Parse(ctx *gin.Context) error {
	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	c.req = TelegramChatRequest{TelegramChatID: telegramChatID}

	return nil
}

func (c *GetSettingsCommand) Apply(ctx *gin.Context) (any, error) {
	settings, err := c.service.UserSettings(ctx.Request.Context(), c.req.TelegramChatID)
	if err != nil {
		return nil, err
	}

	return SettingsResponse{
		Settings: mapUserSettingsToDTO(settings),
	}, nil
}
