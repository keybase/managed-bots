package reminderscheduler

import (
	"time"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
)

// reference: https://stackoverflow.com/a/39295990
type ReminderTicker struct {
	*time.Timer
}

func NewReminderTicker() *ReminderTicker {
	return &ReminderTicker{time.NewTimer(getNextTickDuration())}
}

func (rt *ReminderTicker) Update() {
	rt.Reset(getNextTickDuration())
}

func getNextTickDuration() time.Duration {
	now := time.Now()
	// the next tick is the following minute from now
	nextTick := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()+1, 0, 0, time.Local)
	return nextTick.Sub(now)
}

func (r *ReminderScheduler) sendReminderLoop(shutdownCh chan struct{}) error {
	ticker := NewReminderTicker()
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
			ticker.Update()
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
				if subscription.MinutesBefore == 0 {
					r.ChatEcho(subscription.KeybaseConvID, "An event is starting now: %s", event.MsgContent)
				} else {
					r.ChatEcho(subscription.KeybaseConvID, "An event is starting in %s: %s",
						gcalbot.MinutesBeforeString(subscription.MinutesBefore), event.MsgContent)
				}
			}
		}
		remove()
		// TODO(marcel): remove event if this was the last reminder
	})
	r.reminderSchedule.Delete(timestamp)
}
