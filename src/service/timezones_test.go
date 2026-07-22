//nolint:goconst // Tests intentionally repeat timezone fixture values.
package service_test

import (
	"testing"

	"github.com/EgorKo25/EasyNote/src/service"
	"github.com/stretchr/testify/require"
)

func TestTimezoneOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   func() service.TimezoneOption
		expected service.TimezoneOption
	}{
		{
			name: "timezone by index returns first utc option",
			action: func() service.TimezoneOption {
				return service.TimezoneByIndex(0)
			},
			expected: service.TimezoneOption{City: "UTC-11", Timezone: "Etc/GMT+11"},
		},
		{
			name: "timezone by index returns kaliningrad option",
			action: func() service.TimezoneOption {
				return service.TimezoneByIndex(13)
			},
			expected: service.TimezoneOption{City: "Калининград", Timezone: "Europe/Kaliningrad"},
		},
		{
			name: "timezone by index returns moscow option",
			action: func() service.TimezoneOption {
				return service.TimezoneByIndex(service.TimezoneIndex("Europe/Moscow"))
			},
			expected: service.TimezoneOption{City: "Москва", Timezone: "Europe/Moscow"},
		},
		{
			name: "timezone shift returns next option",
			action: func() service.TimezoneOption {
				return service.TimezoneByIndex(service.TimezoneIndexByShift(14, 1))
			},
			expected: service.TimezoneOption{City: "UTC+04", Timezone: "Etc/GMT-4"},
		},
		{
			name: "unknown timezone falls back to moscow",
			action: func() service.TimezoneOption {
				return service.TimezoneByIndex(service.TimezoneIndex("Asia/Novosibirsk"))
			},
			expected: service.TimezoneOption{City: "Москва", Timezone: "Europe/Moscow"},
		},
		{
			name: "negative timezone shift wraps to last option",
			action: func() service.TimezoneOption {
				return service.TimezoneByIndex(service.TimezoneIndexByShift(0, -1))
			},
			expected: service.TimezoneOption{City: "UTC+12", Timezone: "Etc/GMT-12"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := test.action()

			require.Equal(t, test.expected, actual)
		})
	}
}

func TestTimezoneOptionsReturnsCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   func() service.TimezoneOption
		expected service.TimezoneOption
	}{
		{
			name: "returns first option before mutation",
			action: func() service.TimezoneOption {
				return service.TimezoneOptions()[0]
			},
			expected: service.TimezoneOption{City: "UTC-11", Timezone: "Etc/GMT+11"},
		},
		{
			name: "returns moscow option before mutation",
			action: func() service.TimezoneOption {
				return service.TimezoneOptions()[14]
			},
			expected: service.TimezoneOption{City: "Москва", Timezone: "Europe/Moscow"},
		},
		{
			name: "returns last option before mutation",
			action: func() service.TimezoneOption {
				options := service.TimezoneOptions()

				return options[len(options)-1]
			},
			expected: service.TimezoneOption{City: "UTC+12", Timezone: "Etc/GMT-12"},
		},
		{
			name: "returns copied first option after local mutation",
			action: func() service.TimezoneOption {
				options := service.TimezoneOptions()
				options[0] = service.TimezoneOption{City: "broken", Timezone: "broken"}

				return service.TimezoneOptions()[0]
			},
			expected: service.TimezoneOption{City: "UTC-11", Timezone: "Etc/GMT+11"},
		},
		{
			name: "returns copied moscow option after local mutation",
			action: func() service.TimezoneOption {
				options := service.TimezoneOptions()
				options[14] = service.TimezoneOption{City: "broken", Timezone: "broken"}

				return service.TimezoneOptions()[14]
			},
			expected: service.TimezoneOption{City: "Москва", Timezone: "Europe/Moscow"},
		},
		{
			name: "returns copied last option after local mutation",
			action: func() service.TimezoneOption {
				options := service.TimezoneOptions()
				options[len(options)-1] = service.TimezoneOption{City: "broken", Timezone: "broken"}

				nextOptions := service.TimezoneOptions()

				return nextOptions[len(nextOptions)-1]
			},
			expected: service.TimezoneOption{City: "UTC+12", Timezone: "Etc/GMT-12"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := test.action()

			require.Equal(t, test.expected, actual)
		})
	}
}
