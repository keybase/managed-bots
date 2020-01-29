package reminderscheduler

import (
	"context"
	"fmt"
	"strconv"
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
	newCalEvent *calendar.Event,
	subscriptionSet *gcalbot.AggregatedReminderSubscription,
) error {
	if newCalEvent.Start.DateTime == "" {
		// TODO(marcel): notifications for all day events
		return nil
	}
	newStartTime, err := time.Parse(time.RFC3339, newCalEvent.Start.DateTime)
	if err != nil {
		return fmt.Errorf("error parsing event start time: %s", err)
	}

	event, ok := r.eventMap.Get(newCalEvent.Id)

	timezone, err := srv.Settings.Get("timezone").Do()
	if err != nil {
		return err
	}
	format24HourTimeSetting, err := srv.Settings.Get("format24HourTime").Do()
	if err != nil {
		return err
	}
	format24HourTime, err := strconv.ParseBool(format24HourTimeSetting.Value)
	if err != nil {
		return err
	}
	invitedCalendar, err := srv.Calendars.Get(subscriptionSet.CalendarID).Do()
	if err != nil {
		return err
	}
	eventMsgContent, err := gcalbot.FormatEvent(newCalEvent, subscriptionSet.Account.AccountNickname,
		invitedCalendar.Summary, timezone.Value, format24HourTime)
	if err != nil {
		return err
	}

	if ok {
		event.Lock()
		defer event.Unlock()
		if !newStartTime.Equal(event.StartTime) {
			event.StartTime = newStartTime
			for index, subscription := range event.Subscriptions {
				// remove old reminder
				r.reminderSchedule.ForEachReminderInMinute(subscription.Timestamp, func(reminderEvent *ReminderEvent, remove func()) {
					reminderEvent.Lock()
					defer reminderEvent.Unlock()
					if event.EventID == reminderEvent.EventID {
						remove()
						r.Debug("removed reminder for %s at %s", newCalEvent.Id, subscription.Timestamp)
					}
				})
				// add new reminder
				newTimestamp := getReminderTimestamp(newStartTime, subscription.MinutesBefore)
				r.reminderSchedule.AddReminderToMinute(newTimestamp, event)
				r.Debug("added reminder for %s at %s", newCalEvent.Id, newTimestamp)
				// update subscription
				event.Subscriptions[index].Timestamp = newTimestamp
				r.Debug("updated subscription for %s to", newCalEvent.Id, newTimestamp)
			}
		}
		event.MsgContent = eventMsgContent
	} else {
		newEvent := &ReminderEvent{
			EventID:    newCalEvent.Id,
			StartTime:  newStartTime,
			MsgContent: eventMsgContent,
		}
		newEvent.Lock()
		defer newEvent.Unlock()
		r.eventMap.Set(newCalEvent.Id, newEvent)
		for _, minutesBefore := range subscriptionSet.MinutesBefore {
			if newStartTime.Add(-time.Duration(minutesBefore) * time.Minute).Before(time.Now()) {
				continue
			}
			timestamp := getReminderTimestamp(newStartTime, minutesBefore)
			newEvent.Subscriptions = append(newEvent.Subscriptions, &ReminderEventSubscriptions{
				KeybaseConvID: subscriptionSet.KeybaseConvID,
				Timestamp:     timestamp,
				MinutesBefore: minutesBefore,
			})
			r.Debug("added reminder for %s at %s", newCalEvent.Id, timestamp)
			r.reminderSchedule.AddReminderToMinute(timestamp, newEvent)
		}
	}

	return nil
}
