package reminderscheduler

import (
	"time"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
)

func (r *ReminderScheduler) sendReminderLoop(shutdownCh chan struct{}) error {
	// sleep until the next minute so that the loop executes at the beginning of each minute
	now := time.Now()
	nextMinute := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()+1, 0, 0, time.Local)
	time.Sleep(nextMinute.Sub(now))

	ticker := time.NewTicker(time.Minute)
	defer func() {
		ticker.Stop()
		r.Debug("shutting down sendReminderLoop")
	}()
	for {
		select {
		case <-shutdownCh:
			return nil
		case sendMinute := <-ticker.C:
			r.sendReminders(sendMinute)
			sendDuration := time.Since(sendMinute)
			if sendDuration.Seconds() > 15 {
				r.Errorf("sending reminders took %s", sendDuration.String())
			}
		}
	}
}

func (r *ReminderScheduler) sendReminders(sendMinute time.Time) {
	timestamp := getReminderTimestamp(sendMinute, 0)
	r.reminderSchedule.ForEachReminderInMinute(timestamp, func(event *ReminderEvent, remove func()) {
		event.Lock()
		defer event.Unlock()
		for _, subscription := range event.Subscriptions {
			if subscription.Timestamp == timestamp {
				// TODO(marcel): check if the user still has this reminder set
				minutesBefore := gcalbot.GetMinutesFromDuration(subscription.DurationBefore)
				if minutesBefore == 0 {
					r.ChatEcho(subscription.KeybaseConvID, "An event is starting now: %s", event.MsgContent)
				} else {
					r.ChatEcho(subscription.KeybaseConvID, "An event is starting in %s: %s",
						gcalbot.MinutesBeforeString(minutesBefore), event.MsgContent)
				}
			}
		}
		remove()
		// TODO(marcel): remove event if this was the last reminder
	})
	r.reminderSchedule.Delete(timestamp)
}
