package commands

import (
	"fmt"
	"strconv"

	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func parseTelegramChatRequest(ctx *gin.Context) (TelegramChatRequest, error) {
	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return TelegramChatRequest{}, err
	}

	return TelegramChatRequest{TelegramChatID: telegramChatID}, nil
}

func parsePaginationRequest(ctx *gin.Context) (PaginationRequest, error) {
	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return PaginationRequest{}, err
	}

	req := PaginationRequest{
		TelegramChatID: telegramChatID,
		Limit:          10,
		Offset:         0,
	}

	if value := ctx.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return PaginationRequest{}, apphttp.ErrInvalidOffset
		}

		req.Offset = offset
	}

	if value := ctx.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			return PaginationRequest{}, apphttp.ErrInvalidLimit
		}

		req.Limit = limit
	}

	return req, nil
}

func parseReminderRequest(ctx *gin.Context) (ReminderRequest, error) {
	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return ReminderRequest{}, err
	}

	reminderID, err := parsePositiveInt64Param(ctx, "id", apphttp.ErrInvalidID)
	if err != nil {
		return ReminderRequest{}, err
	}

	return ReminderRequest{
		TelegramChatID: telegramChatID,
		ReminderID:     reminderID,
	}, nil
}

func parseHolidayRequest(ctx *gin.Context) (HolidayRequest, error) {
	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return HolidayRequest{}, err
	}

	holidayID, err := parsePositiveInt64Param(ctx, "id", apphttp.ErrInvalidID)
	if err != nil {
		return HolidayRequest{}, err
	}

	return HolidayRequest{
		TelegramChatID: telegramChatID,
		HolidayID:      holidayID,
	}, nil
}

func parseShoppingListRequest(ctx *gin.Context) (ShoppingListRequest, error) {
	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return ShoppingListRequest{}, err
	}

	listID, err := parsePositiveInt64Param(ctx, "id", apphttp.ErrInvalidID)
	if err != nil {
		return ShoppingListRequest{}, err
	}

	return ShoppingListRequest{
		TelegramChatID: telegramChatID,
		ListID:         listID,
	}, nil
}

func parseToggleShoppingListItemRequest(ctx *gin.Context) (ToggleShoppingListItemRequest, error) {
	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return ToggleShoppingListItemRequest{}, err
	}

	itemID, err := parsePositiveInt64Param(ctx, "item_id", apphttp.ErrInvalidItemID)
	if err != nil {
		return ToggleShoppingListItemRequest{}, err
	}

	return ToggleShoppingListItemRequest{
		TelegramChatID: telegramChatID,
		ItemID:         itemID,
	}, nil
}

func parseAcknowledgeReminderRequest(ctx *gin.Context) (AcknowledgeReminderRequest, error) {
	reminderRequest, err := parseReminderRequest(ctx)
	if err != nil {
		return AcknowledgeReminderRequest{}, err
	}

	return AcknowledgeReminderRequest(reminderRequest), nil
}

func telegramChatIDFromContext(ctx *gin.Context) (int64, error) {
	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return 0, fmt.Errorf("read telegram chat id: %w", err)
	}

	return telegramChatID, nil
}

func parsePositiveInt64Param(ctx *gin.Context, name string, invalidErr error) (int64, error) {
	value, err := strconv.ParseInt(ctx.Param(name), 10, 64)
	if err != nil || value <= 0 {
		return 0, invalidErr
	}

	return value, nil
}
