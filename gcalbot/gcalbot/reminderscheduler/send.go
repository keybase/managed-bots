package reminderscheduler

import (
	"time"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
)

func (r *ReminderScheduler) sendReminderLoop(shutdownCh chan struct{}) error {
	// sleep until the next minute so that the loop executes at the beginning of each minute
	now := time.Now()
	nextMinute := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()+1, 0, 0, time.Local)

	select {
	case <-shutdownCh:
		return nil
	case <-time.After(nextMinute.Sub(now)):
	}

	ticker := time.NewTicker(time.Minute)
	defer func() {
		ticker.Stop()
		r.Debug("shutting down sendReminderLoop")
	}()

	r.sendReminders(time.Now())
	for {
		select {
		case <-shutdownCh:
			return nil
		case sendMinute := <-ticker.C:
			r.sendReminders(sendMinute)
		}
	}
}

func (r *ReminderScheduler) sendReminders(sendMinute time.Time) {
	timestamp := getReminderTimestamp(sendMinute, 0)
	r.minuteReminders.ForEachReminderMessageInMinute(timestamp, func(msg *ReminderMessage) {
		for duration := range msg.MinuteReminders {
			msgTimestamp := getReminderTimestamp(msg.StartTime, duration)
			if msgTimestamp == timestamp {
				minutesBefore := gcalbot.GetMinutesFromDuration(duration)
				if minutesBefore == 0 {
					r.ChatEcho(msg.KeybaseConvID, "An event is starting now: %s", msg.MsgContent)
				} else {
					r.ChatEcho(msg.KeybaseConvID, "An event is starting in %s: %s",
						gcalbot.MinutesBeforeString(minutesBefore), msg.MsgContent)
				}
				delete(msg.MinuteReminders, duration)
				r.stats.Count("sendReminders - reminder")
			}
		}
		if len(msg.MinuteReminders) == 0 {
			r.subscriptionReminders.RemoveReminderMessageFromSubscription(msg)
			r.eventReminders.RemoveReminderMessageFromEvent(msg)
			r.Debug("removed event with no reminders %s", msg.EventID)
			// the entire minute will be removed, and since this is the event's last minute there is no need to delete 'all' minutes
		}
	})
	r.minuteReminders.RemoveMinute(timestamp)
	sendDuration := time.Since(sendMinute)
	if sendDuration.Seconds() > 15 {
		r.Errorf("sending reminders took %s", sendDuration.String())
	}
	r.stats.Value("sendReminders - duration - seconds", sendDuration.Seconds())
}
