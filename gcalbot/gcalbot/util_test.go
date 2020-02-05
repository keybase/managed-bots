package gcalbot_test

import (
	"testing"
	"time"

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
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-01T18:30:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-01-01T19:30:00-05:00",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("same year and month, different day", func(t *testing.T) {
		expected := "Wed Jan 1 4:30pm - Thu Jan 2, 2020 6:30pm (EST)"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-01T16:30:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-01-02T18:30:00-05:00",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("same year, different month and day", func(t *testing.T) {
		expected := "Fri Jan 31 5pm - Sat Feb 1, 2020 6pm (EST)"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-31T17:00:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-02-01T18:00:00-05:00",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("different year, month and day", func(t *testing.T) {
		expected := "Thu Dec 31, 2020 8:30am - Fri Jan 1, 2021 9:30am (EST)"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-12-31T08:30:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2021-01-01T09:30:00-05:00",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day same year, month and day", func(t *testing.T) {
		expected := "Wed Jan 1, 2020"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-01",
			},
			&calendar.EventDateTime{
				Date: "2020-01-02",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day same year and month, different day", func(t *testing.T) {
		expected := "Wed Jan 1 - Thu Jan 2, 2020"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-01",
			},
			&calendar.EventDateTime{
				Date: "2020-01-03",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day same year, different month and day", func(t *testing.T) {
		expected := "Fri Jan 31 - Sat Feb 1, 2020"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-31",
			},
			&calendar.EventDateTime{
				Date: "2020-02-02",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day different year, month and day", func(t *testing.T) {
		expected := "Thu Dec 31, 2020 - Fri Jan 1, 2021"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-12-31",
			},
			&calendar.EventDateTime{
				Date: "2021-01-02",
			},
			timezone,
			false,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("24 hour format", func(t *testing.T) {
		expected := "Fri Jan 31 17:00 - Sat Feb 1, 2020 18:00 (EST)"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				DateTime: "2020-01-31T17:00:00-05:00",
			},
			&calendar.EventDateTime{
				DateTime: "2020-02-01T18:00:00-05:00",
			},
			timezone,
			true,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("all day 24 hour format", func(t *testing.T) {
		expected := "Fri Jan 31 - Sat Feb 1, 2020"
		timezone, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		actual, err := gcalbot.FormatTimeRange(
			&calendar.EventDateTime{
				Date: "2020-01-31",
			},
			&calendar.EventDateTime{
				Date: "2020-02-02",
			},
			timezone,
			true,
		)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

func TestFormatMinuteString(t *testing.T) {
	require.Equal(t, "0 minutes", gcalbot.FormatMinuteString(0))
	require.Equal(t, "1 minute", gcalbot.FormatMinuteString(1))
	require.Equal(t, "2 minutes", gcalbot.FormatMinuteString(2))

	require.Equal(t, "", gcalbot.FormatMinuteSeriesString(nil))
	require.Equal(t, "", gcalbot.FormatMinuteSeriesString([]int{}))

	require.Equal(t, "0 minute", gcalbot.FormatMinuteSeriesString([]int{0}))
	require.Equal(t, "1 minute", gcalbot.FormatMinuteSeriesString([]int{1}))
	require.Equal(t, "0 and 1 minute", gcalbot.FormatMinuteSeriesString([]int{0, 1}))
	require.Equal(t, "0, 1 and 2 minute", gcalbot.FormatMinuteSeriesString([]int{0, 1, 2}))
	require.Equal(t, "2, 3, 5 and 9 minute", gcalbot.FormatMinuteSeriesString([]int{3, 5, 2, 9}))
}
