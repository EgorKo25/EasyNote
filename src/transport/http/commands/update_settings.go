package commands

import (
	"fmt"
	"net/http"
	"time"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type UpdateSettingsCommand struct {
	req     UpdateSettingsRequest
	service *service.ReminderService
}

func NewUpdateSettingsCommand(svc *service.ReminderService) *UpdateSettingsCommand {
	return &UpdateSettingsCommand{
		service: svc,
	}
}

func (c *UpdateSettingsCommand) Method() string {
	return http.MethodPatch
}

func (c *UpdateSettingsCommand) Path() string {
	return "/settings"
}

func (c *UpdateSettingsCommand) Clone() apphttp.Command {
	return &UpdateSettingsCommand{
		service: c.service,
	}
}

func (c *UpdateSettingsCommand) Parse(ctx *gin.Context) error {
	var req UpdateSettingsRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	if req.Timezone != nil {
		if _, err = time.LoadLocation(*req.Timezone); err != nil {
			return apphttp.ErrSettingsTimezoneInvalid
		}
	}

	if req.IntrusiveIntervalMinutes != nil {
		interval := *req.IntrusiveIntervalMinutes
		if interval < 1 || interval > 120 {
			return apphttp.ErrIntrusiveIntervalInvalid
		}
	}

	req.TelegramChatID = telegramChatID
	c.req = req

	return nil
}

func (c *UpdateSettingsCommand) Apply(ctx *gin.Context) (any, error) {
	settings, err := c.service.UserSettings(ctx.Request.Context(), c.req.TelegramChatID)
	if err != nil {
		return nil, err
	}

	if c.req.Timezone != nil {
		if err = c.service.UpdateUserTimezone(ctx.Request.Context(), c.req.TelegramChatID, *c.req.Timezone); err != nil {
			return nil, err
		}
	}

	if c.req.IntrusiveEnabled != nil || c.req.IntrusiveIntervalMinutes != nil {
		intrusiveEnabled := settings.IntrusiveEnabled
		intrusiveInterval := settings.IntrusiveInterval

		if c.req.IntrusiveEnabled != nil {
			intrusiveEnabled = *c.req.IntrusiveEnabled
		}

		if c.req.IntrusiveIntervalMinutes != nil {
			intrusiveInterval = time.Duration(*c.req.IntrusiveIntervalMinutes) * time.Minute
		}

		if err = c.service.UpdateIntrusiveSettings(
			ctx.Request.Context(),
			c.req.TelegramChatID,
			intrusiveEnabled,
			intrusiveInterval,
		); err != nil {
			return nil, err
		}
	}

	settings, err = c.service.UserSettings(ctx.Request.Context(), c.req.TelegramChatID)
	if err != nil {
		return nil, err
	}

	return SettingsResponse{
		Settings: mapUserSettingsToDTO(settings),
	}, nil
}
