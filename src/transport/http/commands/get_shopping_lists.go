package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetShoppingListsCommand struct {
	req     PaginationRequest
	service *service.ReminderService
}

const getShoppingListsPath = "/shopping-lists"

func NewGetShoppingListsCommand(svc *service.ReminderService) *GetShoppingListsCommand {
	return &GetShoppingListsCommand{
		service: svc,
	}
}

func (c *GetShoppingListsCommand) Method() string {
	return http.MethodGet
}

func (c *GetShoppingListsCommand) Path() string {
	return getShoppingListsPath
}

func (c *GetShoppingListsCommand) Clone() apphttp.Command {
	return &GetShoppingListsCommand{
		service: c.service,
	}
}

func (c *GetShoppingListsCommand) Parse(ctx *gin.Context) error {
	req, err := parsePaginationRequest(ctx)
	if err != nil {
		return err
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
		return nil, fmt.Errorf("find shopping lists by chat: %w", err)
	}

	return ShoppingListsResponse{
		ShoppingLists: mapShoppingListsViewToResponse(lists),
	}, nil
}
