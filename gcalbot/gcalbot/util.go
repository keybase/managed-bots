package gcalbot

import (
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
)

func FormatTimeRange(
	startDateTime, endDateTime *calendar.EventDateTime,
	timezoneID string, format24HourTime bool,
) (timeRange string, err error) {
	// For normal events:
	//	If the year, month and day are the same: Wed Jan 1, 2020 6:30pm - 7:30pm (EST)
	//	If just the year and month are the same: Wed Jan 1 4:30pm - Thu Jan 2, 2020 6:30pm (EST)
	//	If just the year is the same (same ^):   Fri Jan 31 5pm - Sat Feb 1, 2020 6pm (EST)
	//	If none of the params are the same:		 Thu Dec 31, 2020 8:30am - Fri Jan 1, 2021 9:30am (EST)
	// For all day:
	//	If the year, month and day are the same: Wed Jan 1, 2020
	//	If just the year and month are the same: Wed Jan 1 - Thu Jan 2, 2020
	//	If just the year is the same (same ^):   Fri Jan 31 - Sat Feb 1, 2020
	//	If none of the params are the same:		 Thu Dec 31, 2020 - Fri Jan 1, 2021

	var isAllDay bool
	var start, end time.Time
	if startDateTime.DateTime != "" && endDateTime.DateTime != "" {
		// this is a normal event
		isAllDay = false
		start, err = time.Parse(time.RFC3339, startDateTime.DateTime)
		if err != nil {
			return "", err
		}
		end, err = time.Parse(time.RFC3339, endDateTime.DateTime)
		if err != nil {
			return "", err
		}
		// set timezone
		location, err := time.LoadLocation(timezoneID)
		if err != nil {
			return "", err
		}
		start = start.In(location)
		end = end.In(location)
	} else if startDateTime.Date != "" && endDateTime.Date != "" {
		// this is an all day event
		isAllDay = true
		start, err = time.Parse("2006-01-02", startDateTime.Date)
		if err != nil {
			return "", err
		}
		end, err = time.Parse("2006-01-02", endDateTime.Date)
		if err != nil {
			return "", err
		}
		end = end.Add(-24 * time.Hour) // the google API sets the end day to the day after, so compensate by one day
	} else {
		return "", fmt.Errorf("invalid dates: start: %+v, end: %+v", startDateTime, endDateTime)
	}

	startYear, startMonth, startDay := start.Date()
	endYear, endMonth, endDay := end.Date()

	var startTime string
	var endTime string
	if !isAllDay {
		if format24HourTime {
			startTime = start.Format("15:04")
			endTime = end.Format("15:04")
		} else {
			if start.Minute() == 0 {
				startTime = start.Format("3pm")
			} else {
				startTime = start.Format("3:04pm")
			}
			if end.Minute() == 0 {
				endTime = end.Format("3pm")
			} else {
				endTime = end.Format("3:04pm")
			}
		}
	}

	if startYear == endYear && startMonth == endMonth && startDay == endDay {
		if isAllDay {
			return start.Format("Mon Jan 2, 2006"), nil
		} else {
			return fmt.Sprintf("%s %s - %s (%s)",
				start.Format("Mon Jan 2, 2006"), startTime, endTime, start.Format("MST")), nil
		}
	} else if startYear == endYear {
		if isAllDay {
			return fmt.Sprintf("%s - %s",
				start.Format("Mon Jan 2"), end.Format("Mon Jan 2, 2006")), nil
		} else {
			return fmt.Sprintf("%s %s - %s %s (%s)",
				start.Format("Mon Jan 2"), startTime, end.Format("Mon Jan 2, 2006"), endTime,
				start.Format("MST")), nil
		}
	} else {
		if isAllDay {
			return fmt.Sprintf("%s - %s",
				start.Format("Mon Jan 2, 2006"), end.Format("Mon Jan 2, 2006")), nil
		} else {
			return fmt.Sprintf("%s %s - %s %s (%s)",
				start.Format("Mon Jan 2, 2006"), startTime, end.Format("Mon Jan 2, 2006"), endTime,
				start.Format("MST")), nil
		}
	}
}

// TODO(marcel): implement backoff for api calls
//func GoogleAPIBackoff(operation func() error) error {
//	var operationErr error
//	err := backoff.Retry(func() error {
//		operationErr = operation()
//		switch err := operationErr.(type) {
//		case *googleapi.Error:
//			switch err.Code {
//			case 403, 404, 500:
//				return err // fail (keep trying)
//			}
//			return nil // retrying won't fix the error, return
//		default:
//			return operationErr // fail (keep trying)
//		}
//	}, backoff.NewExponentialBackOff())
//	if err != nil {
//		return err
//	}
//	return operationErr
//}

// TODO(marcel): sanitize method for kb messages
