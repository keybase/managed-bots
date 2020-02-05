package reminderscheduler

import (
	"container/list"
	"context"
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
		subscriptions, err := r.db.GetAggregatedReminderSubscriptionsWithToken()
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
				SingleEvents(true).
				Pages(context.Background(), func(page *calendar.Events) error {
					events = append(events, page.Items...)
					return nil
				})
			if err != nil {
				r.Errorf("error getting events from API: %s", err)
				continue
			}
			for _, event := range events {
				err = r.UpdateOrCreateReminderEvent(srv, event, &subscription.AggregatedSubscription)
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

func (r *ReminderScheduler) UpdateOrCreateReminderEvent(
	srv *calendar.Service,
	event *calendar.Event,
	subscriptionSet *gcalbot.AggregatedSubscription,
) error {
	status := gcalbot.EventStatus(event.Status)
	if status == gcalbot.EventStatusCancelled {
		r.Debug("removed cancelled event %s", event.Summary)
		r.removeEventByID(event.Id)
		return nil
	}

	start, _, isAllDay, err := gcalbot.ParseTime(event.Start, event.End)
	if err != nil {
		return err
	}

	if isAllDay {
		// TODO(marcel): notifications for all day events
		return nil
	}

	var reminderMessage *ReminderMessage
	r.eventReminders.ForEachReminderMessageInEvent(event.Id, func(msg *ReminderMessage) {
		if msg.AccountID == subscriptionSet.AccountID &&
			msg.CalendarID == subscriptionSet.CalendarID &&
			msg.KeybaseConvID == subscriptionSet.KeybaseConvID {
			// TODO(marcel): figure out how to break
			reminderMessage = msg
		}
	})

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

	if reminderMessage != nil {
		reminderMessage.Lock()
		defer reminderMessage.Unlock()

		if !start.Equal(reminderMessage.StartTime) {
			// remove all minutes
			r.minuteReminders.RemoveReminderMessageFromAllMinutes(reminderMessage)
			// update the start time, and then all of the minutes
			reminderMessage.StartTime = start
			for _, duration := range subscriptionSet.DurationBefore {
				if time.Now().Before(start.Add(-duration)) {
					r.minuteReminders.AddReminderMessageToMinute(duration, reminderMessage)
					r.Debug("added a %s reminder for event %s at %s",
						gcalbot.FormatMinuteString(gcalbot.GetMinutesFromDuration(duration)),
						event.Summary,
						getReminderTimestamp(start, duration))
				}
			}
		}

		// update the event
		reminderMessage.MsgContent = eventMsgContent
	} else {
		// create the event
		reminderMessage = &ReminderMessage{
			EventID:         event.Id,
			AccountID:       subscriptionSet.AccountID,
			CalendarID:      subscriptionSet.CalendarID,
			KeybaseConvID:   subscriptionSet.KeybaseConvID,
			StartTime:       start,
			MsgContent:      eventMsgContent,
			MinuteReminders: make(map[time.Duration]*list.Element),
		}
		reminderMessage.Lock()
		defer reminderMessage.Unlock()

		r.subscriptionReminders.AddReminderMessageToSubscription(reminderMessage)
		r.eventReminders.AddReminderMessageToEvent(reminderMessage)
		for _, duration := range subscriptionSet.DurationBefore {
			if time.Now().Before(start.Add(-duration)) {
				r.minuteReminders.AddReminderMessageToMinute(duration, reminderMessage)
				r.Debug("added a %s reminder for event %s at %s",
					gcalbot.FormatMinuteString(gcalbot.GetMinutesFromDuration(duration)),
					event.Summary,
					getReminderTimestamp(start, duration))
			}
		}
	}

	// check if there are any minutes set, if not remove the event
	if len(reminderMessage.MinuteReminders) == 0 {
		r.Debug("removed event with no reminders %s", event.Summary)
		r.subscriptionReminders.RemoveReminderMessageFromSubscription(reminderMessage)
		r.eventReminders.RemoveReminderMessageFromEvent(reminderMessage)
	}

	return nil
}

func (r *ReminderScheduler) removeEventByID(eventID string) {
	r.eventReminders.ForEachReminderMessageInEvent(eventID, func(msg *ReminderMessage) {
		r.subscriptionReminders.RemoveReminderMessageFromSubscription(msg)
		r.minuteReminders.RemoveReminderMessageFromAllMinutes(msg)
	})
	r.eventReminders.RemoveEvent(eventID)
}

func (r *ReminderScheduler) AddSubscription(subscription gcalbot.Subscription) {
	r.subscriptionReminders.ForEachReminderMessageInSubscription(
		subscription.AccountID, subscription.CalendarID, subscription.KeybaseConvID,
		func(msg *ReminderMessage, removeReminderMessageFromSubscription func()) {
			r.Debug("added %s reminder for event %s at %s",
				gcalbot.FormatMinuteString(gcalbot.GetMinutesFromDuration(subscription.DurationBefore)),
				msg.EventID,
				getReminderTimestamp(msg.StartTime, subscription.DurationBefore))
			r.minuteReminders.AddReminderMessageToMinute(subscription.DurationBefore, msg)
		})
}

func (r *ReminderScheduler) RemoveSubscription(subscription gcalbot.Subscription) {
	r.subscriptionReminders.ForEachReminderMessageInSubscription(
		subscription.AccountID, subscription.CalendarID, subscription.KeybaseConvID,
		func(msg *ReminderMessage, removeReminderMessageFromSubscription func()) {
			r.minuteReminders.RemoveReminderMessageFromMinute(msg, subscription.DurationBefore)
			r.Debug("removed %s reminder for event %s at %s",
				gcalbot.FormatMinuteString(gcalbot.GetMinutesFromDuration(subscription.DurationBefore)),
				msg.EventID,
				getReminderTimestamp(msg.StartTime, subscription.DurationBefore))
			if len(msg.MinuteReminders) == 0 {
				r.Debug("removed event with no reminders %s", msg.EventID)
				r.eventReminders.RemoveReminderMessageFromEvent(msg)
				removeReminderMessageFromSubscription()
			}
		})
}
