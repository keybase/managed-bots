package reminderscheduler

import "time"

func getReminderTimestamp(start time.Time, minutesBefore int) string {
	return start.
		UTC().
		Add(-time.Duration(minutesBefore) * time.Minute).
		Round(time.Minute).
		Format(time.RFC3339)
}
