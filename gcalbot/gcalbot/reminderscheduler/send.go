package reminderscheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"github.com/keybase/pipeliner"
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
	var sendTasks []struct {
		convID  chat1.ConvIDStr
		message string
	}
	r.minuteReminders.ForEachReminderMessageInMinute(timestamp, func(msg *ReminderMessage) {
		for duration := range msg.MinuteReminders {
			msgTimestamp := getReminderTimestamp(msg.StartTime, duration)
			if msgTimestamp == timestamp {
				minutesBefore := gcalbot.GetMinutesFromDuration(duration)
				var eventSummary string
				if msg.EventSummary != "" {
					eventSummary = fmt.Sprintf(`"%s"`, msg.EventSummary)
				} else {
					eventSummary = "An event"
				}
				var message string
				if minutesBefore == 0 {
					message = fmt.Sprintf("%s is starting now: %s", eventSummary, msg.MsgContent)
				} else {
					message = fmt.Sprintf("%s is starting in %s: %s",
						eventSummary, gcalbot.MinutesBeforeString(minutesBefore), msg.MsgContent)
				}
				sendTasks = append(sendTasks, struct {
					convID  chat1.ConvIDStr
					message string
				}{msg.KeybaseConvID, message})
				delete(msg.MinuteReminders, duration)
				r.stats.Count("sendReminders - reminder")
			}
		}
		if len(msg.MinuteReminders) == 0 {
			r.subscriptionReminders.RemoveReminderMessageFromSubscription(msg)
			r.eventReminders.RemoveReminderMessageFromEvent(msg)
			// the entire minute will be removed, and since this is the event's last minute there is no need to delete 'all' minutes
		}
	})
	r.minuteReminders.RemoveMinute(timestamp)

	const sendWindow = 10
	ctx := context.Background()
	pipe := pipeliner.NewPipeliner(sendWindow)
	worker := func(_ context.Context, i int) error { // nolint:unparam
		t := sendTasks[i]
		r.ChatEcho(t.convID, "%s", t.message)
		return nil
	}
	for i := range sendTasks {
		if err := pipe.WaitForRoom(ctx); err != nil {
			break
		}
		go func() { pipe.CompleteOne(worker(ctx, i)) }()
	}
	_ = pipe.Flush(ctx)

	sendDuration := time.Since(sendMinute)
	if sendDuration.Seconds() > 15 {
		r.Errorf("sending %d reminders took %s", len(sendTasks), sendDuration.String())
	}
	r.stats.Value("sendReminders - duration - seconds", sendDuration.Seconds())
}
