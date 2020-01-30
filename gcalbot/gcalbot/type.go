package gcalbot

import "google.golang.org/api/calendar/v3"

type ReminderScheduler interface {
	UpdateOrCreateReminderEvent(
		srv *calendar.Service,
		event *calendar.Event,
		subscriptionSet *AggregatedSubscription,
	) error
}
