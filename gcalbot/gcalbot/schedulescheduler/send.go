package schedulescheduler

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
)

func (s *ScheduleScheduler) sendDailyScheduleLoop(shutdownCh chan struct{}) error {
	// sleep until the next half hour so that the loop executes at the beginning of each half hour
	now := time.Now()
	nextHalfHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 30*(1+now.Minute()/30), 0, 0, time.Local)

	select {
	case <-shutdownCh:
		return nil
	case <-time.After(nextHalfHour.Sub(now)):
	}

	ticker := time.NewTicker(30 * time.Minute)
	defer func() {
		ticker.Stop()
		s.Debug("shutting down sendDailyScheduleLoop")
	}()

	s.sendDailySchedule(time.Now().UTC(), shutdownCh)
	for {
		select {
		case <-shutdownCh:
			return nil
		case sendMinute := <-ticker.C:
			s.sendDailySchedule(sendMinute.UTC(), shutdownCh)
		}
	}
}

func (s *ScheduleScheduler) sendDailySchedule(sendMinute time.Time, shutdownCh chan struct{}) {
	dayStart := time.Date(sendMinute.Year(), sendMinute.Month(), sendMinute.Day(), 0, 0, 0, 0, time.UTC)
	notificationDuration := sendMinute.Sub(dayStart).Round(time.Minute)

	subscriptions, err := s.db.GetAggregatedDailyScheduleSubscriptionByDuration(notificationDuration)
	if err != nil {
		s.Errorf("error getting daily schedule subscriptions to sync: %s", err)
	}
	for _, subscription := range subscriptions {
		select {
		case <-shutdownCh:
			return
		default:
		}

		srv, err := gcalbot.GetCalendarService(&subscription.Account, s.oauth)
		if err != nil {
			s.Errorf(err.Error())
			continue
		}

		timezone, err := gcalbot.GetUserTimezone(srv)
		if err != nil {
			s.Errorf("unable to get user timezone: %s", err)
			continue
		}

		format24HourTime, err := gcalbot.GetUserFormat24HourTime(srv)
		if err != nil {
			s.Errorf("unable to get user 24 hour time setting: %s", err)
			continue
		}

		// TODO(marcel): respect users set date
		userSendMinute := sendMinute.In(timezone)
		minTime := time.Date(userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day(), 0, 0, 0, 0, timezone)
		maxTime := minTime.Add(24 * time.Hour)

		var events []*calendar.Event
		for _, calendarID := range subscription.CalendarIDs {
			err = srv.Events.
				List(calendarID).
				TimeMin(minTime.Format(time.RFC3339)).
				TimeMax(maxTime.Format(time.RFC3339)).
				SingleEvents(true).
				OrderBy("startTime").
				Pages(context.Background(), func(page *calendar.Events) error {
					events = append(events, page.Items...)
					return nil
				})
			if err != nil {
				s.Errorf("error getting events from API: %s", err)
				continue
			}
		}

		if len(events) == 0 {
			s.ChatEcho(subscription.KeybaseConvID, "You have no events in account '%s' for today.",
				subscription.Account.AccountNickname)
		} else {
			formattedSchedule, err := gcalbot.FormatEventSchedule(events, timezone, format24HourTime)
			if err != nil {
				s.Errorf("unable to format schedule: %s", err)
				continue
			}
			link := fmt.Sprintf("https://calendar.google.com/calendar/r/day/%d/%d/%d", userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day())
			s.ChatEcho(subscription.KeybaseConvID, "Today for account '%s' - %s\n%s\n%s",
				subscription.Account.AccountNickname, userSendMinute.Format("Monday, January 2"), link, formattedSchedule)
		}
	}

	s.stats.Value("sendDailySchedule - duration - seconds", time.Since(sendMinute).Seconds())
}
