package gcalbot

import (
	"fmt"
	"time"
)

func FormatTimeRange(start, end time.Time) string {
	// If the year, month and day are the same: Wed Jan 1, 2020 6:30pm - 7:30pm (EST)
	// If just the year and month are the same: Wed Jan 1 4:30pm - Thu Jan 2, 2020 6:30pm (EST)
	// If just the year is the same (same ^):   Fri Jan 31 5pm - Sat Feb 1, 2020 6pm (EST)
	// If none of the params are the same:		Thu Dec 31, 2020 8:30am - Fri Jan 1, 2021 9:30am (EST)

	startYear, startMonth, startDay := start.Date()
	endYear, endMonth, endDay := end.Date()

	var startTime string
	if start.Minute() == 0 {
		startTime = start.Format("3pm")
	} else {
		startTime = start.Format("3:04pm")
	}

	var endTime string
	if end.Minute() == 0 {
		endTime = end.Format("3pm")
	} else {
		endTime = end.Format("3:04pm")
	}

	startTimezone, _ := start.Zone()
	// endTimezone, _ := start.Zone()
	// TODO(marcel): handle different timezones as a possibility

	if startYear == endYear && startMonth == endMonth && startDay == endDay {
		return fmt.Sprintf("%s %s - %s (%s)",
			start.Format("Mon Jan 2, 2006"), startTime, endTime, startTimezone)
	} else if startYear == endYear {
		return fmt.Sprintf("%s %s - %s %s (%s)",
			start.Format("Mon Jan 2"), startTime, end.Format("Mon Jan 2, 2006"), endTime,
			startTimezone)
	} else {
		return fmt.Sprintf("%s %s - %s %s (%s)",
			start.Format("Mon Jan 2, 2006"), startTime, end.Format("Mon Jan 2, 2006"), endTime,
			startTimezone)
	}
}
