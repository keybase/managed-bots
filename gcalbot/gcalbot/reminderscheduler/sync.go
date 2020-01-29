package reminderscheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func (r *ReminderScheduler) eventSyncLoop(shutdownCh chan struct{}) error {
	ticker := time.NewTicker(time.Hour)
	defer func() {
		ticker.Stop()
		r.Debug("shutting down eventSyncLoop")
	}()

	eventSync := func() {
		subscriptions, err := r.db.GetAggregatedReminderSubscriptions()
		if err != nil {
			r.Errorf("error getting reminder subscriptions to sync: %s", err)
		}
		for _, subscription := range subscriptions {
			select {
			case <-shutdownCh:
				return
			default:
			}

			client := r.config.Client(context.Background(), subscription.Token)
			srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
			if err != nil {
				r.Errorf(err.Error())
				continue
			}

			// sync 3 hours of events
			minTime := time.Now().UTC()
			maxTime := minTime.Add(3 * time.Hour)
			var events []*calendar.Event
			err = srv.Events.
				List(subscription.CalendarID).
				TimeMin(minTime.Format(time.RFC3339)).
				TimeMax(maxTime.Format(time.RFC3339)).
				Pages(context.Background(), func(page *calendar.Events) error {
					events = append(events, page.Items...)
					return nil
				})
			if err != nil {
				r.Errorf("error getting events from API: %s", err)
				continue
			}
			for _, event := range events {
				err = r.updateOrCreateReminderEvent(srv, event, subscription)
				if err != nil {
					r.Errorf("error updating or creating reminder event: %s", err)
				}
			}
		}
	}

	eventSync()
	for {
		select {
		case <-shutdownCh:
			return nil
		case <-ticker.C:
			eventSync()
		}
	}
}

func (r *ReminderScheduler) updateOrCreateReminderEvent(
	srv *calendar.Service,
	event *calendar.Event,
	subscriptionSet *gcalbot.AggregatedReminderSubscription,
) error {
	if event.Start.DateTime == "" {
		// TODO(marcel): notifications for all day events
		return nil
	}

	newStartTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
	if err != nil {
		return fmt.Errorf("error parsing event start time: %s", err)
	}

	timezone, err := gcalbot.GetUserTimezone(srv)
	if err != nil {
		return err
	}
	format24HourTime, err := gcalbot.GetUserFormat24HourTime(srv)
	if err != nil {
		return err
	}
	subscribedCalendar, err := srv.Calendars.Get(subscriptionSet.CalendarID).Do()
	if err != nil {
		return err
	}
	eventMsgContent, err := gcalbot.FormatEvent(event, subscriptionSet.Account.AccountNickname,
		subscribedCalendar.Summary, timezone, format24HourTime)
	if err != nil {
		return err
	}

	reminderEvent, ok := r.eventMap.Get(event.Id)
	if !ok {
		reminderEvent = &ReminderEvent{
			EventID:    event.Id,
			StartTime:  newStartTime,
			MsgContent: eventMsgContent,
		}
	}

	reminderEvent.Lock()
	defer reminderEvent.Unlock()

	if ok {
		if !newStartTime.Equal(reminderEvent.StartTime) {
			reminderEvent.StartTime = newStartTime
			for index, subscription := range reminderEvent.Subscriptions {
				// remove old reminder
				r.reminderSchedule.ForEachReminderInMinute(subscription.Timestamp, func(minuteEvent *ReminderEvent, remove func()) {
					minuteEvent.Lock()
					defer minuteEvent.Unlock()
					if minuteEvent.EventID == reminderEvent.EventID {
						remove()
						r.Debug("removed reminder for '%s', cal '%s' at %s", event.Summary, subscribedCalendar.Summary, subscription.Timestamp)
					}
				})
				// add new reminder
				newTimestamp := getReminderTimestamp(newStartTime, subscription.DurationBefore)
				r.reminderSchedule.AddReminderToMinute(newTimestamp, reminderEvent)
				r.Debug("added reminder for '%s', cal '%s' at %s", event.Summary, subscribedCalendar.Summary, newTimestamp)
				// update subscription
				reminderEvent.Subscriptions[index].Timestamp = newTimestamp
				r.Debug("updated subscription for '%s', cal '%s' at %s", event.Summary, subscribedCalendar.Summary, newTimestamp)
			}
		}
		reminderEvent.MsgContent = eventMsgContent
	} else {
		r.eventMap.Set(event.Id, reminderEvent)
		for _, durationBefore := range subscriptionSet.DurationBefore {
			if newStartTime.Add(-durationBefore).Before(time.Now()) {
				continue
			}
			timestamp := getReminderTimestamp(newStartTime, durationBefore)
			reminderEvent.Subscriptions = append(reminderEvent.Subscriptions, &ReminderEventSubscriptions{
				KeybaseConvID:  subscriptionSet.KeybaseConvID,
				Timestamp:      timestamp,
				DurationBefore: durationBefore,
			})
			r.Debug("added reminder for '%s', cal '%s' at %s", event.Summary, subscribedCalendar.Summary, timestamp)
			r.reminderSchedule.AddReminderToMinute(timestamp, reminderEvent)
		}
	}

	return nil
}
