package commands

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type CreateShoppingListCommand struct {
	req     CreateShoppingListRequest
	service *service.ReminderService
}

func NewCreateShoppingListCommand(svc *service.ReminderService) *CreateShoppingListCommand {
	return &CreateShoppingListCommand{
		service: svc,
	}
}

func (c *CreateShoppingListCommand) Method() string {
	return http.MethodPost
}

func (c *CreateShoppingListCommand) Path() string {
	return "/shopping-lists"
}

func (c *CreateShoppingListCommand) Clone() apphttp.Command {
	return &CreateShoppingListCommand{
		service: c.service,
	}
}

func (c *CreateShoppingListCommand) Parse(ctx *gin.Context) error {
	var req CreateShoppingListRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %w", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return err
	}

	if len(req.Items) == 0 {
		return apphttp.ErrShoppingListItemsEmpty
	}

	for _, item := range req.Items {
		if strings.TrimSpace(item.Text) == "" {
			return apphttp.ErrShoppingListItemTextEmpty
		}
	}

	req.TelegramChatID = telegramChatID
	c.req = req

	return nil
}

func (c *CreateShoppingListCommand) Apply(ctx *gin.Context) (any, error) {
	list, err := c.service.CreateShoppingList(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		mapCreateShoppingListItemsToDomainItems(c.req.Items),
	)
	if err != nil {
		return nil, fmt.Errorf("create shopping list: %w", err)
	}

	return mapShoppingListViewToResponse(list), nil
}
