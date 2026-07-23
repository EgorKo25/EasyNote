package ui

import (
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/service"
	telegrammodel "github.com/EgorKo25/EasyNote/src/telegram/model"
)

type Draft struct {
	Kind          DraftKind
	Title         string
	Description   string
	Notice        string
	ShoppingItems []telegrammodel.ShoppingListItem
	Date          time.Time
	Hour          int
	Minute        int
	Repeat        Repeat
	CustomDates   []time.Time
	WeekdaysMask  int
	IntervalDays  int
	Screen        Screen
	CalendarMonth time.Time
	WaitingFor    Field
}

type TimezoneOption = service.TimezoneOption

func (d Draft) RemindAt(location *time.Location) time.Time {
	if d.Date.IsZero() {
		return time.Time{}
	}

	return time.Date(d.Date.Year(), d.Date.Month(), d.Date.Day(), d.Hour, d.Minute, 0, 0, location)
}

func (d Draft) SelectedTimeText() string {
	return fmt.Sprintf("%02d:%02d", d.Hour, d.Minute)
}
