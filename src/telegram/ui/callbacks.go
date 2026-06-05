package ui

import (
	"fmt"
	"strconv"
	"time"
)

func DateCallback(value time.Time) string {
	return CallbackDatePrefix + value.Format(time.DateOnly)
}

func MonthCallback(value time.Time) string {
	return CallbackMonthPrefix + value.Format("2006-01")
}

func TimeSetCallback(hour, minute int) string {
	return fmt.Sprintf("%s%02d:%02d", CallbackTimeSetPrefix, hour, minute)
}

func TimeShiftCallback(minutes int) string {
	return CallbackTimeShiftPrefix + strconv.Itoa(minutes)
}

func RepeatCallback(value Repeat) string {
	return CallbackRepeatPrefix + string(value)
}

func WeekdayCallback(mask int) string {
	return CallbackWeekdayPrefix + strconv.Itoa(mask)
}
