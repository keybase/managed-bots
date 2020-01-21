package gcalbot_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
)

func TestFormatTimeRange(t *testing.T) {
	// If the year, month and day are the same: Wed Jan 1, 2020 6:30pm - 7:30pm (EST)
	// If just the year and month are the same: Wed Jan 1 4:30pm - Thu Jan 2, 2020 6:30pm (EST)
	// If just the year is the same (same ^):   Fri Jan 31 5pm - Sat Feb 1, 2020 6pm (EST)
	// If none of the params are the same:		Thu Dec 31, 2020 8:30am - Fri Jan 1, 2021 9:30am (EST)

	t.Run("same year, month and day", func(t *testing.T) {
		start, err := time.Parse(time.RFC1123, "Wed, 01 Jan 2020 18:30:00 EST")
		require.NoError(t, err)
		end, err := time.Parse(time.RFC1123, "Wed, 01 Jan 2020 19:30:00 EST")
		require.NoError(t, err)
		expected := "Wed Jan 1, 2020 6:30pm - 7:30pm (EST)"
		actual := gcalbot.FormatTimeRange(start, end)
		require.Equal(t, expected, actual)
	})

	t.Run("same year and month, different day", func(t *testing.T) {
		start, err := time.Parse(time.RFC1123, "Wed, 01 Jan 2020 16:30:00 EST")
		require.NoError(t, err)
		end, err := time.Parse(time.RFC1123, "Thu, 02 Jan 2020 18:30:00 EST")
		require.NoError(t, err)
		expected := "Wed Jan 1 4:30pm - Thu Jan 2, 2020 6:30pm (EST)"
		actual := gcalbot.FormatTimeRange(start, end)
		require.Equal(t, expected, actual)
	})

	t.Run("same year, different month and day", func(t *testing.T) {
		start, err := time.Parse(time.RFC1123, "Fri, 31 Jan 2020 17:00:00 EST")
		require.NoError(t, err)
		end, err := time.Parse(time.RFC1123, "Sat, 01 Feb 2020 18:00:00 EST")
		require.NoError(t, err)
		expected := "Fri Jan 31 5pm - Sat Feb 1, 2020 6pm (EST)"
		actual := gcalbot.FormatTimeRange(start, end)
		require.Equal(t, expected, actual)
	})

	t.Run("different year, month and day", func(t *testing.T) {
		start, err := time.Parse(time.RFC1123, "Thu, 31 Dec 2020 08:30:00 EST")
		require.NoError(t, err)
		end, err := time.Parse(time.RFC1123, "Fri, 01 Jan 2021 09:30:00 EST")
		require.NoError(t, err)
		expected := "Thu Dec 31, 2020 8:30am - Fri Jan 1, 2021 9:30am (EST)"
		actual := gcalbot.FormatTimeRange(start, end)
		require.Equal(t, expected, actual)
	})
}
