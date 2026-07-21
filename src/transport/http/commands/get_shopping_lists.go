package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type GetShoppingListsCommand struct {
	req     PaginationRequest
	service *service.ReminderService
}

func NewGetShoppingListsCommand(svc *service.ReminderService) *GetShoppingListsCommand {
	return &GetShoppingListsCommand{
		service: svc,
	}
}

func (c *GetShoppingListsCommand) Method() string {
	return http.MethodGet
}

func (c *GetShoppingListsCommand) Path() string {
	return "/shopping-lists"
}

func (c *GetShoppingListsCommand) Clone() apphttp.Command {
	return &GetShoppingListsCommand{
		service: c.service,
	}
}

func (c *GetShoppingListsCommand) Parse(ctx *gin.Context) error {
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

func (c *GetShoppingListsCommand) Apply(ctx *gin.Context) (any, error) {
	lists, err := c.service.FindShoppingListsByChat(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.Limit,
		c.req.Offset,
	)
	if err != nil {
		return nil, err
	}

	return ShoppingListsResponse{
		ShoppingLists: mapShoppingListsViewToResponse(lists),
	}, nil
}
