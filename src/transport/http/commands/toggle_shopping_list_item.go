//nolint:dupl // Command files intentionally repeat lifecycle boilerplate to keep endpoints explicit.
package commands

import (
	"fmt"
	"net/http"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type ToggleShoppingListItemCommand struct {
	req     ToggleShoppingListItemRequest
	service *service.ReminderService
}

const toggleShoppingListItemPath = "/shopping-list-items/:item_id/toggle"

func NewToggleShoppingListItemCommand(svc *service.ReminderService) *ToggleShoppingListItemCommand {
	return &ToggleShoppingListItemCommand{
		service: svc,
	}
}

func (c *ToggleShoppingListItemCommand) Method() string {
	return http.MethodPost
}

func (c *ToggleShoppingListItemCommand) Path() string {
	return toggleShoppingListItemPath
}

func (c *ToggleShoppingListItemCommand) Clone() apphttp.Command {
	return &ToggleShoppingListItemCommand{
		service: c.service,
	}
}

func (c *ToggleShoppingListItemCommand) Parse(ctx *gin.Context) error {
	req, err := parseToggleShoppingListItemRequest(ctx)
	if err != nil {
		return err
	}

	c.req = req

	return nil
}

func (c *ToggleShoppingListItemCommand) Apply(ctx *gin.Context) (any, error) {
	list, err := c.service.ToggleShoppingListItem(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.ItemID,
	)
	if err != nil {
		return nil, fmt.Errorf("toggle shopping list item: %w", err)
	}

	return mapShoppingListViewToResponse(list), nil
}
