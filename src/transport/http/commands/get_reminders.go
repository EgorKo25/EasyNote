package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	transporthttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type GetRemindersCommand struct {
	req     GetReminderRequest
	service *service.ReminderService
}

type GetReminderRequest struct {
	Offset         int
	Limit          int
	TelegramChatID int64
}

type GetRemindersResponse struct {
	Reminders []ReminderDTO `json:"reminders"`
}

func NewGetRemindersCommand(svc *service.ReminderService) *GetRemindersCommand {
	return &GetRemindersCommand{
		service: svc,
	}
}

func (c *GetRemindersCommand) Method() string {
	return http.MethodGet
}

func (c *GetRemindersCommand) Path() string {
	return "/reminders"
}

func (c *GetRemindersCommand) Clone() transporthttp.Command {
	return &GetRemindersCommand{
		service: c.service,
	}
}

func (c *GetRemindersCommand) Parse(ctx *gin.Context) error {
	var req GetReminderRequest

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	req.TelegramChatID = telegramChatID
	req.Limit = 10
	req.Offset = 0

	if value := ctx.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil {
			return ErrInvalidOffset
		}
		req.Offset = normalizeOffset(offset)
	}

	if value := ctx.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			return ErrInvalidLimit
		}
		req.Limit = limit
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
		return nil, err
	}

	return GetRemindersResponse{
		Reminders: mapRemindersViewToResponse(reminders),
	}, nil
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		offset = 0
	}

	return offset
}
