package service

type TimezoneOption struct {
	City     string
	Timezone string
}

func TimezoneOptions() []TimezoneOption {
	return append([]TimezoneOption(nil), timezoneOptions...)
}

func TimezoneIndex(timezone string) int {
	for index, option := range timezoneOptions {
		if option.Timezone == timezone {
			return index
		}
	}

	return moscowTimezoneIndex()
}

func TimezoneByIndex(index int) TimezoneOption {
	return timezoneOptions[normalizeTimezoneIndex(index)]
}

func TimezoneIndexByShift(index int, shift int) int {
	return normalizeTimezoneIndex(index + shift)
}

func normalizeTimezoneIndex(index int) int {
	count := len(timezoneOptions)
	if count == 0 {
		return 0
	}

	index %= count
	if index < 0 {
		index += count
	}

	return index
}

func moscowTimezoneIndex() int {
	for index, option := range timezoneOptions {
		if option.Timezone == "Europe/Moscow" {
			return index
		}
	}

	return 0
}

var timezoneOptions = []TimezoneOption{
	{City: "UTC-11", Timezone: "Etc/GMT+11"},
	{City: "UTC-10", Timezone: "Etc/GMT+10"},
	{City: "UTC-09", Timezone: "Etc/GMT+9"},
	{City: "UTC-08", Timezone: "Etc/GMT+8"},
	{City: "UTC-07", Timezone: "Etc/GMT+7"},
	{City: "UTC-06", Timezone: "Etc/GMT+6"},
	{City: "UTC-05", Timezone: "Etc/GMT+5"},
	{City: "UTC-04", Timezone: "Etc/GMT+4"},
	{City: "UTC-03", Timezone: "Etc/GMT+3"},
	{City: "UTC-02", Timezone: "Etc/GMT+2"},
	{City: "UTC-01", Timezone: "Etc/GMT+1"},
	{City: "UTC+00", Timezone: "Etc/GMT"},
	{City: "UTC+01", Timezone: "Etc/GMT-1"},
	{City: "Калининград", Timezone: "Europe/Kaliningrad"},
	{City: "Москва", Timezone: "Europe/Moscow"},
	{City: "UTC+04", Timezone: "Etc/GMT-4"},
	{City: "UTC+05", Timezone: "Etc/GMT-5"},
	{City: "UTC+06", Timezone: "Etc/GMT-6"},
	{City: "UTC+07", Timezone: "Etc/GMT-7"},
	{City: "UTC+08", Timezone: "Etc/GMT-8"},
	{City: "UTC+09", Timezone: "Etc/GMT-9"},
	{City: "UTC+10", Timezone: "Etc/GMT-10"},
	{City: "UTC+11", Timezone: "Etc/GMT-11"},
	{City: "UTC+12", Timezone: "Etc/GMT-12"},
}
