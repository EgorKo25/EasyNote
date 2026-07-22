//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type GetShoppingListCommand struct {
	req     ShoppingListRequest
	service *service.ReminderService
}

func NewGetShoppingListCommand(svc *service.ReminderService) *GetShoppingListCommand {
	return &GetShoppingListCommand{
		service: svc,
	}
}

func (c *GetShoppingListCommand) Method() string {
	return http.MethodGet
}

func (c *GetShoppingListCommand) Path() string {
	return "/shopping-lists/:id"
}

func (c *GetShoppingListCommand) Clone() apphttp.Command {
	return &GetShoppingListCommand{
		service: c.service,
	}
}

func (c *GetShoppingListCommand) Parse(ctx *gin.Context) error {
	req, err := parseShoppingListRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *GetShoppingListCommand) Apply(ctx *gin.Context) (any, error) {
	list, err := c.service.ShoppingListByID(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.ListID,
	)
	if err != nil {
		return nil, fmt.Errorf("get shopping list by id: %w", err)
	}

	return mapShoppingListViewToResponse(list), nil
}
