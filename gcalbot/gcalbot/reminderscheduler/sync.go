package reminderscheduler

import (
	"container/list"
	"context"
	"time"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"google.golang.org/api/calendar/v3"
)

func (r *ReminderScheduler) eventSyncLoop(shutdownCh chan struct{}) error {
	ticker := time.NewTicker(time.Hour)
	defer func() {
		ticker.Stop()
		r.Debug("shutting down eventSyncLoop")
	}()

	eventSync := func(syncMinute time.Time) {
		pairs, err := r.db.GetReminderSubscriptionAndAccountPairs()
		r.stats.ValueInt("eventSyncLoop - subscriptions - count", len(pairs))
		if err != nil {
			r.Errorf("error getting reminder subscriptions to sync: %s", err)
		}
		for _, pair := range pairs {
			select {
			case <-shutdownCh:
				return
			default:
			}

			srv, err := gcalbot.GetCalendarService(&pair.Account, r.oauth)
			if err != nil {
				r.Errorf(err.Error())
				continue
			}

			// sync 3 hours of events
			minTime := time.Now().UTC()
			maxTime := minTime.Add(3 * time.Hour)
			var events []*calendar.Event
			err = srv.Events.
				List(pair.Subscription.CalendarID).
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
				err = r.UpdateOrCreateReminderEvent(&pair.Account, &pair.Subscription, event)
				if err != nil {
					r.Errorf("error updating or creating reminder event: %s", err)
				}
			}
		}
		r.stats.Value("eventSyncLoop - duration - seconds", time.Since(syncMinute).Seconds())
	}

	eventSync(time.Now())
	for {
		select {
		case <-shutdownCh:
			return nil
		case syncMinute := <-ticker.C:
			eventSync(syncMinute)
		}
	}
}

func (r *ReminderScheduler) UpdateOrCreateReminderEvent(
	account *gcalbot.Account,
	subscription *gcalbot.Subscription,
	event *calendar.Event,
) error {
	r.stats.Count("UpdateOrCreateReminderEvent")
	status := gcalbot.EventStatus(event.Status)
	if status == gcalbot.EventStatusCancelled {
		if r.eventReminders.ExistsEvent(event.Id) {
			r.stats.Count("UpdateOrCreateReminderEvent - cancel")
			r.Debug("removed cancelled event %s", event.Summary)
			r.removeEventByID(event.Id)
		}
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
		if msg.KeybaseUsername == account.KeybaseUsername &&
			msg.AccountNickname == account.AccountNickname &&
			msg.CalendarID == subscription.CalendarID &&
			msg.KeybaseConvID == subscription.KeybaseConvID {
			// TODO(marcel): figure out how to break
			reminderMessage = msg
		}
	})

	srv, err := gcalbot.GetCalendarService(account, r.oauth)
	if err != nil {
		return err
	}

	timezone, err := gcalbot.GetUserTimezone(srv)
	if err != nil {
		return err
	}
	format24HourTime, err := gcalbot.GetUserFormat24HourTime(srv)
	if err != nil {
		return err
	}
	subscribedCalendar, err := srv.Calendars.Get(subscription.CalendarID).Do()
	if err != nil {
		return err
	}
	eventMsgContent, err := gcalbot.FormatEvent(event, account.AccountNickname,
		subscribedCalendar.Summary, timezone, format24HourTime)
	if err != nil {
		return err
	}

	if reminderMessage != nil {
		// update the event
		r.stats.Count("UpdateOrCreateReminderEvent - update")
		reminderMessage.Lock()
		defer reminderMessage.Unlock()

		if !start.Equal(reminderMessage.StartTime) {
			// remove all minutes
			r.minuteReminders.RemoveReminderMessageFromAllMinutes(reminderMessage)
			// update the start time, and then all of the minutes
			reminderMessage.StartTime = start
			duration := subscription.DurationBefore
			if time.Now().Before(start.Add(-duration)) {
				r.minuteReminders.AddReminderMessageToMinute(duration, reminderMessage)
				r.Debug("added a %s reminder for event %s at %s",
					gcalbot.MinutesBeforeString(gcalbot.GetMinutesFromDuration(duration)),
					event.Summary,
					getReminderTimestamp(start, duration))
			}
		}

		reminderMessage.MsgContent = eventMsgContent
	} else {
		// create the event
		r.stats.Count("UpdateOrCreateReminderEvent - create")
		reminderMessage = &ReminderMessage{
			EventID:         event.Id,
			KeybaseUsername: account.KeybaseUsername,
			AccountNickname: account.AccountNickname,
			CalendarID:      subscription.CalendarID,
			KeybaseConvID:   subscription.KeybaseConvID,
			StartTime:       start,
			MsgContent:      eventMsgContent,
			MinuteReminders: make(map[time.Duration]*list.Element),
		}
		reminderMessage.Lock()
		defer reminderMessage.Unlock()

		r.subscriptionReminders.AddReminderMessageToSubscription(reminderMessage)
		r.eventReminders.AddReminderMessageToEvent(reminderMessage)
		duration := subscription.DurationBefore
		if time.Now().Before(start.Add(-duration)) {
			r.minuteReminders.AddReminderMessageToMinute(duration, reminderMessage)
			r.Debug("added a %s reminder for event %s at %s",
				gcalbot.MinutesBeforeString(gcalbot.GetMinutesFromDuration(duration)),
				event.Summary,
				getReminderTimestamp(start, duration))
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

func (r *ReminderScheduler) AddSubscription(account *gcalbot.Account, subscription gcalbot.Subscription) {
	r.stats.Count("AddSubscription")
	r.subscriptionReminders.ForEachReminderMessageInSubscription(
		account.KeybaseUsername, account.AccountNickname, subscription.CalendarID, subscription.KeybaseConvID,
		func(msg *ReminderMessage, removeReminderMessageFromSubscription func()) {
			r.Debug("added %s reminder for event %s at %s",
				gcalbot.MinutesBeforeString(gcalbot.GetMinutesFromDuration(subscription.DurationBefore)),
				msg.EventID,
				getReminderTimestamp(msg.StartTime, subscription.DurationBefore))
			r.minuteReminders.AddReminderMessageToMinute(subscription.DurationBefore, msg)
		})
}

func (r *ReminderScheduler) RemoveSubscription(account *gcalbot.Account, subscription gcalbot.Subscription) {
	r.stats.Count("RemoveSubscription")
	r.subscriptionReminders.ForEachReminderMessageInSubscription(
		account.KeybaseUsername, account.AccountNickname, subscription.CalendarID, subscription.KeybaseConvID,
		func(msg *ReminderMessage, removeReminderMessageFromSubscription func()) {
			r.minuteReminders.RemoveReminderMessageFromMinute(msg, subscription.DurationBefore)
			r.Debug("removed %s reminder for event %s at %s",
				gcalbot.MinutesBeforeString(gcalbot.GetMinutesFromDuration(subscription.DurationBefore)),
				msg.EventID,
				getReminderTimestamp(msg.StartTime, subscription.DurationBefore))
			if len(msg.MinuteReminders) == 0 {
				r.Debug("removed event with no reminders %s", msg.EventID)
				r.eventReminders.RemoveReminderMessageFromEvent(msg)
				removeReminderMessageFromSubscription()
			}
		})
}
