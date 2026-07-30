package commands

import (
	"fmt"
	"mime"

	"github.com/EgorKo25/EasyNote/src/domain"
	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

const queryRemindersPath = "/reminders"

const defaultReminderQueryLimit = 10
const queryRemindersMediaType = "application/json"

type QueryRemindersCommand struct {
	req     ReminderQueryRequest
	service *service.ReminderService
}

func NewQueryRemindersCommand(svc *service.ReminderService) *QueryRemindersCommand {
	return &QueryRemindersCommand{service: svc}
}

func (c *QueryRemindersCommand) Method() string {
	return apphttp.MethodQuery
}

func (c *QueryRemindersCommand) Path() string {
	return queryRemindersPath
}

func (c *QueryRemindersCommand) Clone() apphttp.Command {
	return &QueryRemindersCommand{service: c.service}
}

func (c *QueryRemindersCommand) Parse(ctx *gin.Context) error {
	request, err := parseQueryRemindersRequest(ctx)
	if err != nil {
		return err
	}

	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return err
	}

	limit, err := reminderQueryLimit(request.Limit)
	if err != nil {
		return err
	}

	offset, err := reminderQueryOffset(request.Offset)
	if err != nil {
		return err
	}

	c.req = ReminderQueryRequest{
		TelegramChatID: telegramChatID,
		Statuses:       request.Statuses,
		Limit:          limit,
		Offset:         offset,
	}

	return nil
}

func parseQueryRemindersRequest(ctx *gin.Context) (QueryRemindersRequest, error) {
	ctx.Header("Accept-Query", queryRemindersMediaType)

	contentType := ctx.GetHeader("Content-Type")
	if contentType == "" {
		return QueryRemindersRequest{}, apphttp.ErrQueryContentTypeMissing
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != queryRemindersMediaType {
		return QueryRemindersRequest{}, apphttp.ErrQueryContentTypeUnsupported
	}

	var request QueryRemindersRequest
	if err = ctx.ShouldBindJSON(&request); err != nil {
		return QueryRemindersRequest{}, fmt.Errorf("%w: %w", apphttp.ErrInvalidRequest, err)
	}

	if len(request.Statuses) == 0 {
		return QueryRemindersRequest{}, apphttp.ErrReminderStatusesEmpty
	}

	if err = validateReminderStatuses(request.Statuses); err != nil {
		return QueryRemindersRequest{}, err
	}

	return request, nil
}

func reminderQueryLimit(requestLimit *int) (int, error) {
	limit := defaultReminderQueryLimit
	if requestLimit != nil {
		limit = *requestLimit
	}

	if limit < 1 || limit > 100 {
		return 0, apphttp.ErrInvalidLimit
	}

	return limit, nil
}

func reminderQueryOffset(requestOffset *int) (int, error) {
	offset := 0
	if requestOffset != nil {
		offset = *requestOffset
	}

	if offset < 0 {
		return 0, apphttp.ErrInvalidOffset
	}

	return offset, nil
}

func (c *QueryRemindersCommand) Apply(ctx *gin.Context) (any, error) {
	reminders, err := c.service.FindRemindersByStatuses(
		ctx.Request.Context(),
		c.req.TelegramChatID,
		c.req.Statuses,
		c.req.Limit,
		c.req.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query reminders by statuses: %w", err)
	}

	return ReminderListResponse{
		Reminders: mapRemindersViewToResponse(reminders),
	}, nil
}

func validateReminderStatuses(statuses []domain.ReminderStatus) error {
	seen := make(map[domain.ReminderStatus]struct{}, len(statuses))
	for _, status := range statuses {
		switch status {
		case domain.ReminderStatusNew,
			domain.ReminderStatusSent,
			domain.ReminderStatusFailed,
			domain.ReminderStatusCanceled,
			domain.ReminderStatusCompleted,
			domain.ReminderStatusSkipped:
		default:
			return fmt.Errorf("%w: %s", apphttp.ErrReminderStatusInvalid, status)
		}

		if _, exists := seen[status]; exists {
			return fmt.Errorf("%w: duplicate %s", apphttp.ErrReminderStatusInvalid, status)
		}

		seen[status] = struct{}{}
	}

	return nil
}
