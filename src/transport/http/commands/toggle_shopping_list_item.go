package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type ToggleShoppingListItemCommand struct {
	req     ToggleShoppingListItemRequest
	service *service.ReminderService
}

func NewToggleShoppingListItemCommand(svc *service.ReminderService) *ToggleShoppingListItemCommand {
	return &ToggleShoppingListItemCommand{
		service: svc,
	}
}

func (c *ToggleShoppingListItemCommand) Method() string {
	return http.MethodPost
}

func (c *ToggleShoppingListItemCommand) Path() string {
	return "/shopping-list-items/:item_id/toggle"
}

func (c *ToggleShoppingListItemCommand) Clone() apphttp.Command {
	return &ToggleShoppingListItemCommand{
		service: c.service,
	}
}

func (c *ToggleShoppingListItemCommand) Parse(ctx *gin.Context) error {
	var req ToggleShoppingListItemRequest

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	itemID, err := strconv.ParseInt(ctx.Param("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		return apphttp.ErrInvalidItemID
	}

	req.TelegramChatID = telegramChatID
	req.ItemID = itemID
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
		return nil, err
	}

	return mapShoppingListViewToResponse(list), nil
}
