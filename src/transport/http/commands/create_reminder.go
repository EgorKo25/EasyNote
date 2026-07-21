package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	apphttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
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
		return fmt.Errorf("%w: %v", apphttp.ErrInvalidRequest, err)
	}

	telegramChatID, err := middleware.TelegramChatID(ctx)
	if err != nil {
		return err
	}

	request.TelegramChatID = telegramChatID
	c.request = request

	return nil
}

func (c *CreateReminderCommand) Apply(ctx *gin.Context) (any, error) {
	var reminder database.ReminderView
	var err error

	telegramChatID := c.request.TelegramChatID

	if c.request.Repeat == nil {
		reminder, err = c.service.CreateReminder(
			ctx.Request.Context(),
			telegramChatID,
			c.request.Title,
			c.request.Description,
			c.request.RemindAt,
		)
	} else {
		switch c.request.Repeat.Type {
		case string(database.ReminderRuleTypeInterval):
			if c.request.Repeat.IntervalDays == nil {
				return nil, apphttp.ErrReminderRepeatIntervalDaysEmpty
			}
			if *c.request.Repeat.IntervalDays < 1 {
				return nil, apphttp.ErrReminderRepeatIntervalDaysInvalid
			}

			reminder, err = c.service.CreateIntervalReminder(
				ctx.Request.Context(),
				telegramChatID,
				c.request.Title,
				c.request.Description,
				c.request.RemindAt,
				*c.request.Repeat.IntervalDays,
			)
		case string(database.ReminderRuleTypeWeekdays):
			if c.request.Repeat.WeekdaysMask == nil {
				return nil, apphttp.ErrReminderRepeatWeekdaysMaskEmpty
			}

			reminder, err = c.service.CreateWeekdaysReminder(
				ctx.Request.Context(),
				telegramChatID,
				c.request.Title,
				c.request.Description,
				c.request.RemindAt,
				*c.request.Repeat.WeekdaysMask,
			)
		case string(database.ReminderRuleTypeCustomDates):
			if len(c.request.Repeat.Dates) == 0 {
				return nil, apphttp.ErrReminderRepeatCustomDatesEmpty
			}
			if c.request.Repeat.Hour == nil || c.request.Repeat.Minute == nil {
				return nil, apphttp.ErrReminderRepeatCustomTimeEmpty
			}
			if *c.request.Repeat.Hour < 0 || *c.request.Repeat.Hour > 23 ||
				*c.request.Repeat.Minute < 0 || *c.request.Repeat.Minute > 59 {
				return nil, apphttp.ErrReminderRepeatCustomTimeInvalid
			}
			if strings.TrimSpace(c.request.Repeat.Timezone) == "" {
				return nil, apphttp.ErrReminderRepeatTimezoneInvalid
			}

			location, loadLocationErr := time.LoadLocation(c.request.Repeat.Timezone)
			if loadLocationErr != nil {
				return nil, fmt.Errorf("%w: %s", apphttp.ErrReminderRepeatTimezoneInvalid, c.request.Repeat.Timezone)
			}

			dates, parseDatesErr := parseReminderRepeatDates(c.request.Repeat.Dates)
			if parseDatesErr != nil {
				return nil, parseDatesErr
			}

			reminder, err = c.service.CreateCustomDatesReminder(
				ctx.Request.Context(),
				telegramChatID,
				c.request.Title,
				c.request.Description,
				dates,
				*c.request.Repeat.Hour,
				*c.request.Repeat.Minute,
				location,
			)
		default:
			return nil, fmt.Errorf("%w: %s", apphttp.ErrReminderRepeatTypeUnsupported, c.request.Repeat.Type)
		}
	}

	if err != nil {
		return nil, err
	}

	return mapReminderViewToResponse(reminder), nil
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
