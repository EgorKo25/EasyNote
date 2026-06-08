package ui

import (
	"fmt"
	"time"
)

type Draft struct {
	Title         string
	Description   string
	Notice        string
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

func (d Draft) RemindAt(location *time.Location) time.Time {
	if d.Date.IsZero() {
		return time.Time{}
	}

	return time.Date(d.Date.Year(), d.Date.Month(), d.Date.Day(), d.Hour, d.Minute, 0, 0, location)
}

func (d Draft) SelectedTimeText() string {
	return fmt.Sprintf("%02d:%02d", d.Hour, d.Minute)
}
