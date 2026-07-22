package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/domain"
	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/gin-gonic/gin"
)

type CreateReminderCommand struct {
	request CreateReminderRequest
	service *service.ReminderService
}

func NewCreateReminderCommand(service *service.ReminderService) *CreateReminderCommand {
	return &CreateReminderCommand{service: service}
}

func (c *CreateReminderCommand) Method() string {
	return "POST"
}

func (c *CreateReminderCommand) Path() string {
	return "/reminders"
}

func (c *CreateReminderCommand) Clone() apphttp.Command {
	return &CreateReminderCommand{
		service: c.service,
	}
}

func (c *CreateReminderCommand) Parse(ctx *gin.Context) error {
	var request CreateReminderRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		return fmt.Errorf("%w: %w", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := telegramChatIDFromContext(ctx)
	if err != nil {
		return err
	}

	request.TelegramChatID = telegramChatID
	c.request = request

	return nil
}

func (c *CreateReminderCommand) Apply(ctx *gin.Context) (any, error) {
	if c.request.Repeat == nil {
		reminder, err := c.service.CreateReminder(
			ctx.Request.Context(),
			c.request.TelegramChatID,
			c.request.Title,
			c.request.Description,
			c.request.RemindAt,
		)
		if err != nil {
			return nil, fmt.Errorf("create reminder: %w", err)
		}

		return mapReminderViewToResponse(reminder), nil
	}

	reminder, err := c.createRepeatReminder(ctx)
	if err != nil {
		return nil, err
	}

	return mapReminderViewToResponse(reminder), nil
}

func (c *CreateReminderCommand) createRepeatReminder(ctx *gin.Context) (domain.ReminderView, error) {
	switch c.request.Repeat.Type {
	case string(domain.ReminderRuleTypeInterval):
		return c.createIntervalReminder(ctx)
	case string(domain.ReminderRuleTypeWeekdays):
		return c.createWeekdaysReminder(ctx)
	case string(domain.ReminderRuleTypeCustomDates):
		return c.createCustomDatesReminder(ctx)
	default:
		return domain.ReminderView{}, fmt.Errorf(
			"%w: %s",
			apphttp.ErrReminderRepeatTypeUnsupported,
			c.request.Repeat.Type,
		)
	}
}

func (c *CreateReminderCommand) createIntervalReminder(ctx *gin.Context) (domain.ReminderView, error) {
	if c.request.Repeat.IntervalDays == nil {
		return domain.ReminderView{}, apphttp.ErrReminderRepeatIntervalDaysEmpty
	}

	if *c.request.Repeat.IntervalDays < 1 {
		return domain.ReminderView{}, apphttp.ErrReminderRepeatIntervalDaysInvalid
	}

	reminder, err := c.service.CreateIntervalReminder(
		ctx.Request.Context(),
		c.request.TelegramChatID,
		c.request.Title,
		c.request.Description,
		c.request.RemindAt,
		*c.request.Repeat.IntervalDays,
	)
	if err != nil {
		return domain.ReminderView{}, fmt.Errorf("create interval reminder: %w", err)
	}

	return reminder, nil
}

func (c *CreateReminderCommand) createWeekdaysReminder(ctx *gin.Context) (domain.ReminderView, error) {
	if c.request.Repeat.WeekdaysMask == nil {
		return domain.ReminderView{}, apphttp.ErrReminderRepeatWeekdaysMaskEmpty
	}

	reminder, err := c.service.CreateWeekdaysReminder(
		ctx.Request.Context(),
		c.request.TelegramChatID,
		c.request.Title,
		c.request.Description,
		c.request.RemindAt,
		*c.request.Repeat.WeekdaysMask,
	)
	if err != nil {
		return domain.ReminderView{}, fmt.Errorf("create weekdays reminder: %w", err)
	}

	return reminder, nil
}

func (c *CreateReminderCommand) createCustomDatesReminder(ctx *gin.Context) (domain.ReminderView, error) {
	repeat := c.request.Repeat

	if len(repeat.Dates) == 0 {
		return domain.ReminderView{}, apphttp.ErrReminderRepeatCustomDatesEmpty
	}

	hour, minute, err := parseReminderRepeatTime(repeat.Hour, repeat.Minute)
	if err != nil {
		return domain.ReminderView{}, err
	}

	if strings.TrimSpace(repeat.Timezone) == "" {
		return domain.ReminderView{}, apphttp.ErrReminderRepeatTimezoneInvalid
	}

	location, err := time.LoadLocation(repeat.Timezone)
	if err != nil {
		return domain.ReminderView{}, fmt.Errorf(
			"%w: %s",
			apphttp.ErrReminderRepeatTimezoneInvalid,
			repeat.Timezone,
		)
	}

	dates, err := parseReminderRepeatDates(repeat.Dates)
	if err != nil {
		return domain.ReminderView{}, err
	}

	reminder, err := c.service.CreateCustomDatesReminder(
		ctx.Request.Context(),
		c.request.TelegramChatID,
		c.request.Title,
		c.request.Description,
		dates,
		hour,
		minute,
		location,
	)
	if err != nil {
		return domain.ReminderView{}, fmt.Errorf("create custom dates reminder: %w", err)
	}

	return reminder, nil
}

func parseReminderRepeatTime(hour, minute *int) (int, int, error) {
	if hour == nil || minute == nil {
		return 0, 0, apphttp.ErrReminderRepeatCustomTimeEmpty
	}

	if *hour < 0 || *hour > 23 || *minute < 0 || *minute > 59 {
		return 0, 0, apphttp.ErrReminderRepeatCustomTimeInvalid
	}

	return *hour, *minute, nil
}

func parseReminderRepeatDates(values []string) ([]time.Time, error) {
	dates := make([]time.Time, 0, len(values))
	for _, value := range values {
		date, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", apphttp.ErrReminderRepeatDateInvalid, value)
		}

		dates = append(dates, date)
	}

	return dates, nil
}
