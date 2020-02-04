package reminderscheduler

import "time"

func getReminderTimestamp(start time.Time, durationBefore time.Duration) ReminderTimestamp {
	return ReminderTimestamp(start.
		UTC().
		Add(-durationBefore).
		Round(time.Minute).
		Format(time.RFC3339))
}
