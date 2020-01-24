package gcalbot_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/api/calendar/v3"

	"github.com/stretchr/testify/require"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
)

func TestFormatTimeRange(t *testing.T) {
	// For normal events:
	//	If the year, month and day are the same: Wed Jan 1, 2020 6:30pm - 7:30pm (EST)
	//	If just the year and month are the same: Wed Jan 1 4:30pm - Thu Jan 2, 2020 6:30pm (EST)
	//	If just the year is the same (same ^):   Fri Jan 31 5pm - Sat Feb 1, 2020 6pm (EST)
	//	If none of the params are the same:		 Thu Dec 31, 2020 8:30am - Fri Jan 1, 2021 9:30am (EST)
	// For all day:
	//	If the year, month and day are the same: Wed Jan 1, 2020 (EST)
	//	If just the year and month are the same: Wed Jan 1 - Thu Jan 2, 2020 (EST)
	//	If just the year is the same (same ^):   Fri Jan 31 - Sat Feb 1, 2020 (EST)
	//	If none of the params are the same:		 Thu Dec 31, 2020 - Fri Jan 1, 2021 (EST)

	t.Run("same year, month and day", func(t *testing.T) {
		expected := "Wed Jan 1, 2020 6:30pm - 7:30pm (EST)"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-01T18:30:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-01-01T19:30:00-05:00",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("same year and month, different day", func(t *testing.T) {
		expected := "Wed Jan 1 4:30pm - Thu Jan 2, 2020 6:30pm (EST)"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-01T16:30:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-01-02T18:30:00-05:00",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("same year, different month and day", func(t *testing.T) {
		expected := "Fri Jan 31 5pm - Sat Feb 1, 2020 6pm (EST)"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-31T17:00:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-02-01T18:00:00-05:00",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("different year, month and day", func(t *testing.T) {
		expected := "Thu Dec 31, 2020 8:30am - Fri Jan 1, 2021 9:30am (EST)"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-12-31T08:30:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2021-01-01T09:30:00-05:00",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day same year, month and day", func(t *testing.T) {
		expected := "Wed Jan 1, 2020"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-01",
			},
			&calendar.EventDateTime{
				Date: "2020-01-02",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day same year and month, different day", func(t *testing.T) {
		expected := "Wed Jan 1 - Thu Jan 2, 2020"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-01",
			},
			&calendar.EventDateTime{
				Date: "2020-01-03",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day same year, different month and day", func(t *testing.T) {
		expected := "Fri Jan 31 - Sat Feb 1, 2020"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-31",
			},
			&calendar.EventDateTime{
				Date: "2020-02-02",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day different year, month and day", func(t *testing.T) {
		expected := "Thu Dec 31, 2020 - Fri Jan 1, 2021"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-12-31",
			},
			&calendar.EventDateTime{
				Date: "2021-01-02",
			},
			"America/New_York",
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("24 hour format", func(t *testing.T) {
		expected := "Fri Jan 31 17:00 - Sat Feb 1, 2020 18:00 (EST)"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-31T17:00:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-02-01T18:00:00-05:00",
			},
			"America/New_York",
			true,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day 24 hour format", func(t *testing.T) {
		expected := "Fri Jan 31 - Sat Feb 1, 2020"
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-31",
			},
			&calendar.EventDateTime{
				Date: "2020-02-02",
			},
			"America/New_York",
			true,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

func TestParseReminderDuration(t *testing.T) {
	unitTable := map[string][]string{
		"minutes": {"m", "min", "mins", "minute", "minutes"},
		"hours":   {"h", "hr", "hrs", "hour", "hours"},
		"days":    {"d", "day", "days"},
		"weeks":   {"w", "wk", "wks", "week", "weeks"},
	}

	for unit, suffixList := range unitTable {
		for length := 0; length <= 4; length++ {
			expected := fmt.Sprintf("%d %s", length, unit)
			if length == 1 {
				expected = strings.TrimSuffix(expected, "s")
			}
			for _, suffix := range suffixList {
				duration, userErr, err := gcalbot.ParseReminderDuration(strconv.Itoa(length), suffix)
				require.NoError(t, err)
				require.Empty(t, userErr)
				require.Equal(t, expected, duration.String())
			}
		}
	}
}
