package reminderscheduler

import (
	"container/list"
	"context"
	"errors"
	"time"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

func (r *ReminderScheduler) eventSyncLoop(shutdownCh chan struct{}) error {
	ticker := time.NewTicker(time.Hour)
	defer func() {
		ticker.Stop()
		r.Debug("shutting down eventSyncLoop")
	}()

	eventSync := func(syncMinute time.Time) {
		pairs, err := r.db.GetReminderSubscriptionAndAccountPairs(context.Background())
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
			r.syncEvents(&pair.Account, &pair.Subscription)
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

func (r *ReminderScheduler) syncEvents(account *gcalbot.Account, subscription *gcalbot.Subscription) {
	var err error
	defer func() {
		if err = r.cal.WrapAuth(context.Background(), account, err); err != nil {
			r.Errorf("error syncing events: %s", err)
		}
	}()

	srv, err := r.cal.GetCalendarService(context.Background(), account)
	if err != nil {
		return
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
		var gerr *googleapi.Error
		if errors.As(err, &gerr) && gerr.Code == 404 {
			// Calendar was deleted or the user lost access.
			r.Debug("calendar no longer accessible (404): %s", subscription.CalendarID)
			err = nil
		}
		return
	}
	for _, event := range events {
		err = r.UpdateOrCreateReminderEvent(account, subscription, event)
		if err != nil {
			if gcalbot.IsAccountAuthError(err) {
				return
			}
			r.Errorf("error updating or creating reminder event: %s", err)
			err = nil
		}
	}
}

func (r *ReminderScheduler) UpdateOrCreateReminderEvent(
	account *gcalbot.Account,
	subscription *gcalbot.Subscription,
	event *calendar.Event,
) (err error) {
	defer func() { err = r.cal.InvalidateIfAuthError(context.Background(), account, err) }()
	r.stats.Count("UpdateOrCreateReminderEvent")
	status := gcalbot.EventStatus(event.Status)
	if status == gcalbot.EventStatusCancelled {
		if r.eventReminders.ExistsEvent(event.Id) {
			r.stats.Count("UpdateOrCreateReminderEvent - cancel")
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

	srv, err := r.cal.GetCalendarService(context.Background(), account)
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
	eventMsgContent, err := gcalbot.FormatEvent(event, subscribedCalendar.Summary, timezone, format24HourTime)
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
			}
		}

		reminderMessage.EventSummary = event.Summary
		reminderMessage.MsgContent = eventMsgContent
	} else {
		// create the event
		r.stats.Count("UpdateOrCreateReminderEvent - create")
		reminderMessage = &ReminderMessage{
			EventID:         event.Id,
			EventSummary:    event.Summary,
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
		}
	}

	// check if there are any minutes set, if not remove the event
	if len(reminderMessage.MinuteReminders) == 0 {
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
	if subscription.Type != gcalbot.SubscriptionTypeReminder {
		return
	}
	r.stats.Count("AddSubscription")

	r.subscriptionReminders.ForEachReminderMessageInSubscription(
		account.KeybaseUsername, account.AccountNickname, subscription.CalendarID, subscription.KeybaseConvID,
		func(msg *ReminderMessage, _ func()) {
			r.minuteReminders.AddReminderMessageToMinute(subscription.DurationBefore, msg)
		},
	)

	// do a background sync when a new subscription is added
	go r.syncEvents(account, &subscription)
}

func (r *ReminderScheduler) RemoveSubscription(account *gcalbot.Account, subscription gcalbot.Subscription) {
	if subscription.Type != gcalbot.SubscriptionTypeReminder {
		return
	}
	r.stats.Count("RemoveSubscription")
	r.subscriptionReminders.ForEachReminderMessageInSubscription(
		account.KeybaseUsername, account.AccountNickname, subscription.CalendarID, subscription.KeybaseConvID,
		func(msg *ReminderMessage, removeReminderMessageFromSubscription func()) {
			r.minuteReminders.RemoveReminderMessageFromMinute(msg, subscription.DurationBefore)
			if len(msg.MinuteReminders) == 0 {
				r.eventReminders.RemoveReminderMessageFromEvent(msg)
				removeReminderMessageFromSubscription()
			}
		},
	)
}
