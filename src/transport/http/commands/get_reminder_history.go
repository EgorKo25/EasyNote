package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
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
	var req PaginationRequest

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	req.TelegramChatID = telegramChatID
	req.Limit = 10
	req.Offset = 0

	if value := ctx.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return apphttp.ErrInvalidOffset
		}
		req.Offset = offset
	}

	if value := ctx.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			return apphttp.ErrInvalidLimit
		}
		req.Limit = limit
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
		return nil, err
	}

	return ReminderListResponse{
		Reminders: mapRemindersViewToResponse(reminders),
	}, nil
}
