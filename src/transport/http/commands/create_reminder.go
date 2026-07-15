package commands

import (
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/EgorKo25/EasyNote/src/service"
	transporthttp "github.com/EgorKo25/EasyNote/src/transport/http"
	"github.com/EgorKo25/EasyNote/src/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type CreateReminderCommand struct {
	request CreateReminderRequest
	service *service.ReminderService
}

type CreateReminderRequest struct {
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	RemindAt       time.Time       `json:"remind_at"`
	Repeat         *ReminderRepeat `json:"repeat"`
	TelegramChatID int64           `json:"-"`
}

type ReminderRepeat struct {
	Type         string   `json:"type"`
	IntervalDays *int     `json:"interval_days,omitempty"`
	WeekdaysMask *int     `json:"weekdays_mask,omitempty"`
	Dates        []string `json:"dates"`
	Hour         *int     `json:"hour,omitempty"`
	Minute       *int     `json:"minute,omitempty"`
	Timezone     string   `json:"timezone,omitempty"`
}

type CreateReminderResponse struct {
	Reminder ReminderDTO `json:"reminder"`
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

func (c *CreateReminderCommand) Clone() transporthttp.Command {
	return &CreateReminderCommand{
		service: c.service,
	}
}

func (c *CreateReminderCommand) Parse(ctx *gin.Context) error {
	var request CreateReminderRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
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
				return nil, ErrReminderRepeatIntervalDaysEmpty
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
				return nil, ErrReminderRepeatWeekdaysMaskEmpty
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
			if c.request.Repeat.Hour == nil || c.request.Repeat.Minute == nil {
				return nil, ErrReminderRepeatCustomTimeEmpty
			}

			location, loadLocationErr := time.LoadLocation(c.request.Repeat.Timezone)
			if loadLocationErr != nil {
				return nil, fmt.Errorf("%w: %s", ErrReminderRepeatTimezoneInvalid, c.request.Repeat.Timezone)
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
			return nil, fmt.Errorf("%w: %s", ErrReminderRepeatTypeUnsupported, c.request.Repeat.Type)
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
			return nil, fmt.Errorf("%w: %s", ErrReminderRepeatDateInvalid, value)
		}

		dates = append(dates, date)
	}

	return dates, nil
}
