package commands

import (
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
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
	var req ShoppingListRequest

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	listID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || listID <= 0 {
		return apphttp.ErrInvalidID
	}

	req.TelegramChatID = telegramChatID
	req.ListID = listID
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
		return nil, err
	}

	return mapShoppingListViewToResponse(list), nil
}
